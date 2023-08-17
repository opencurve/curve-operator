package sys

import (
	"fmt"
	"github.com/opencurve/curve-operator/pkg/util/exec"
	"k8s.io/klog"
	osexec "os/exec"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/google/uuid"
)

const (
	// DiskType is a disk type
	DiskType = "disk"
	// PartType is a partition type
	PartType = "part"
	// LVMType is an LVM type
	LVMType   = "lvm"
	sgdiskCmd = "sgdisk"
)

// Partition represents a partition metadata
type Partition struct {
	Name       string
	Size       uint64
	Label      string
	Filesystem string
}

// LocalDisk contains information about an unformatted block device
type LocalDisk struct {
	// Name is the device name
	Name string `json:"name"`
	// Parent is the device parent's name
	Parent string `json:"parent"`
	// HasChildren is whether the device has a children device
	HasChildren bool `json:"hasChildren"`
	// DevLinks is the persistent device path on the host
	DevLinks string `json:"devLinks"`
	// Size is the device capacity in byte
	Size uint64 `json:"size"`
	// UUID is used by /dev/disk/by-uuid
	UUID string `json:"uuid"`
	// Serial is the disk serial used by /dev/disk/by-id
	Serial string `json:"serial"`
	// Type is disk type
	Type string `json:"type"`
	// Rotational is the boolean whether the device is rotational: true for hdd, false for ssd and nvme
	Rotational bool `json:"rotational"`
	// ReadOnly is the boolean whether the device is readonly
	Readonly bool `json:"readOnly"`
	// Partitions is a partition slice
	Partitions []Partition
	// Filesystem is the filesystem currently on the device
	Filesystem string `json:"filesystem"`
	// Mountpoint is the mountpoint of the filesystem's on the device
	Mountpoint string `json:"mountpoint"`
	// Vendor is the device vendor
	Vendor string `json:"vendor"`
	// Model is the device model
	Model string `json:"model"`
	// WWN is the world wide name of the device
	WWN string `json:"wwn"`
	// WWNVendorExtension is the WWN_VENDOR_EXTENSION from udev info
	WWNVendorExtension string `json:"wwnVendorExtension"`
	// Empty checks whether the device is completely empty
	Empty bool `json:"empty"`
	// RealPath is the device pathname behind the PVC, behind /mnt/<pvc>/name
	RealPath string `json:"real-path,omitempty"`
	// KernelName is the kernel name of the device
	KernelName string `json:"kernel-name,omitempty"`
	// Whether this device should be encrypted
	Encrypted bool `json:"encrypted,omitempty"`
}

// ListDevices list all devices available on a machine
func ListDevices(executor exec.Executor) ([]string, error) {
	devices, err := executor.ExecuteCommandWithOutput("lsblk", "--all", "--noheadings", "--list", "--output", "KNAME")
	if err != nil {
		return nil, fmt.Errorf("failed to list all devices: %+v", err)
	}

	return strings.Split(devices, "\n"), nil
}

// GetDevicePartitions gets partitions on a given device
func GetDevicePartitions(device string, executor exec.Executor) (partitions []Partition, unusedSpace uint64, err error) {

	var devicePath string
	splitDevicePath := strings.Split(device, "/")
	if len(splitDevicePath) == 1 {
		devicePath = fmt.Sprintf("/dev/%s", device) //device path for OSD on devices.
	} else {
		devicePath = device //use the exact device path (like /mnt/<pvc-name>) in case of PVC block device
	}

	output, err := executor.ExecuteCommandWithOutput("lsblk", devicePath,
		"--bytes", "--pairs", "--output", "NAME,SIZE,TYPE,PKNAME")
	klog.Infof("Output: %+v", output)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get device %s partitions. %+v", device, err)
	}
	partInfo := strings.Split(output, "\n")
	var deviceSize uint64
	var totalPartitionSize uint64
	for _, info := range partInfo {
		props := parseKeyValuePairString(info)
		name := props["NAME"]
		if name == device {
			// found the main device
			klog.Info("Device found - ", name)
			deviceSize, err = strconv.ParseUint(props["SIZE"], 10, 64)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to get device %s size. %+v", device, err)
			}
		} else if props["PKNAME"] == device && props["TYPE"] == PartType {
			// found a partition
			p := Partition{Name: name}
			p.Size, err = strconv.ParseUint(props["SIZE"], 10, 64)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to get partition %s size. %+v", name, err)
			}
			totalPartitionSize += p.Size

			info, err := GetUdevInfo(name, executor)
			if err != nil {
				return nil, 0, err
			}
			if v, ok := info["PARTNAME"]; ok {
				p.Label = v
			}
			if v, ok := info["ID_PART_ENTRY_NAME"]; ok {
				p.Label = v
			}
			if v, ok := info["ID_FS_TYPE"]; ok {
				p.Filesystem = v
			}

			partitions = append(partitions, p)
		}
	}

	if deviceSize > 0 {
		unusedSpace = deviceSize - totalPartitionSize
	}
	return partitions, unusedSpace, nil
}

// GetDeviceProperties gets device properties
func GetDeviceProperties(device string, executor exec.Executor) (map[string]string, error) {
	// As we are mounting the block mode PVs on /mnt we use the entire path,
	// e.g., if the device path is /mnt/example-pvc then its taken completely
	// else if its just vdb then the following is used
	devicePath := strings.Split(device, "/")
	if len(devicePath) == 1 {
		device = fmt.Sprintf("/dev/%s", device)
	}
	return GetDevicePropertiesFromPath(device, executor)
}

// GetDevicePropertiesFromPath gets a device property from a path
func GetDevicePropertiesFromPath(devicePath string, executor exec.Executor) (map[string]string, error) {
	output, err := executor.ExecuteCommandWithOutput("lsblk", devicePath,
		"--bytes", "--nodeps", "--pairs", "--paths", "--output", "SIZE,ROTA,RO,TYPE,PKNAME,NAME,KNAME,MOUNTPOINT,FSTYPE")
	if err != nil {
		klog.Errorf("failed to execute lsblk. output: %s", output)
		return nil, err
	}
	klog.Infof("lsblk output: %q", output)

	return parseKeyValuePairString(output), nil
}

// GetUdevInfo gets udev information
func GetUdevInfo(device string, executor exec.Executor) (map[string]string, error) {
	output, err := executor.ExecuteCommandWithOutput("udevadm", "info", "--query=property", fmt.Sprintf("/dev/%s", device))
	if err != nil {
		return nil, err
	}
	klog.Infof("udevadm info output: %q", output)

	return parseUdevInfo(output), nil
}

// GetDeviceFilesystems get the file systems available
func GetDeviceFilesystems(device string, executor exec.Executor) (string, error) {
	devicePath := strings.Split(device, "/")
	if len(devicePath) == 1 {
		device = fmt.Sprintf("/dev/%s", device)
	}
	output, err := executor.ExecuteCommandWithOutput("udevadm", "info", "--query=property", device)
	if err != nil {
		return "", err
	}

	return parseFS(output), nil
}

// GetDiskUUID look up the UUID for a disk.
func GetDiskUUID(device string, executor exec.Executor) (string, error) {
	if _, err := osexec.LookPath(sgdiskCmd); err != nil {
		return "", errors.Wrap(err, "sgdisk not found")
	}

	devicePath := strings.Split(device, "/")
	if len(devicePath) == 1 {
		device = fmt.Sprintf("/dev/%s", device)
	}

	output, err := executor.ExecuteCommandWithOutput(sgdiskCmd, "--print", device)
	if err != nil {
		return "", errors.Wrapf(err, "sgdisk failed. output=%s", output)
	}

	return parseUUID(device, output)
}

// finds the disk uuid in the output of sgdisk
func parseUUID(device, output string) (string, error) {

	// find the line with the uuid
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// If GPT is not found in a disk, sgdisk creates a new GPT in memory and reports its UUID.
		// This ID changes each call and is not appropriate to identify the device.
		if strings.Contains(line, "Creating new GPT entries in memory.") {
			break
		}
		if strings.Contains(line, "Disk identifier (GUID)") {
			words := strings.Split(line, " ")
			for _, word := range words {
				// we expect most words in the line not to be a uuid, but will return the first one that is
				result, err := uuid.Parse(word)
				if err == nil {
					return result.String(), nil
				}
			}
		}
	}

	return "", fmt.Errorf("uuid not found for device %s. output=%s", device, output)
}

// converts a raw key value pair string into a map of key value pairs
// example raw string of `foo="0" bar="1" baz="biz"` is returned as:
// map[string]string{"foo":"0", "bar":"1", "baz":"biz"}
func parseKeyValuePairString(propsRaw string) map[string]string {
	// first split the single raw string on spaces and initialize a map of
	// a length equal to the number of pairs
	props := strings.Split(propsRaw, " ")
	propMap := make(map[string]string, len(props))

	for _, kvpRaw := range props {
		// split each individual key value pair on the equals sign
		kvp := strings.Split(kvpRaw, "=")
		if len(kvp) == 2 {
			// first element is the final key, second element is the final value
			// (don't forget to remove surrounding quotes from the value)
			propMap[kvp[0]] = strings.Replace(kvp[1], `"`, "", -1)
		}
	}

	return propMap
}

// find fs from udevadm info
func parseFS(output string) string {
	m := parseUdevInfo(output)
	if v, ok := m["ID_FS_TYPE"]; ok {
		return v
	}
	return ""
}

func parseUdevInfo(output string) map[string]string {
	lines := strings.Split(output, "\n")
	result := make(map[string]string, len(lines))
	for _, v := range lines {
		pairs := strings.Split(v, "=")
		if len(pairs) > 1 {
			result[pairs[0]] = pairs[1]
		}
	}
	return result
}

// ListDevicesChild list all child available on a device
// For an encrypted device, it will return the encrypted device like so:
// lsblk --noheadings --output NAME --path --list /dev/sdd
// /dev/sdd
// /dev/mapper/ocs-deviceset-thin-1-data-0hmfgp-block-dmcrypt
func ListDevicesChild(executor exec.Executor, device string) ([]string, error) {
	childListRaw, err := executor.ExecuteCommandWithOutput("lsblk", "--noheadings", "--path", "--list", "--output", "NAME", device)
	if err != nil {
		return []string{}, fmt.Errorf("failed to list child devices of %q. %v", device, err)
	}

	return strings.Split(childListRaw, "\n"), nil
}
