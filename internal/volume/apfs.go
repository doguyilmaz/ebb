package volume

import (
	"encoding/json"
	"fmt"
	"slices"
)

type apfsList struct {
	Containers []struct {
		ContainerReference string `json:"ContainerReference"`
		CapacityCeiling    uint64 `json:"CapacityCeiling"`
		CapacityFree       uint64 `json:"CapacityFree"`
		Volumes            []struct {
			DeviceIdentifier string   `json:"DeviceIdentifier"`
			Name             string   `json:"Name"`
			Roles            []string `json:"Roles"`
			CapacityInUse    uint64   `json:"CapacityInUse"`
		} `json:"Volumes"`
	} `json:"Containers"`
}

// applyAPFS fills container and per-volume use from `diskutil apfs list`
// for the container that holds the data volume dev.
func applyAPFS(b *Breakdown, raw []byte, dev string) error {
	var l apfsList
	if err := json.Unmarshal(raw, &l); err != nil {
		return err
	}
	for _, c := range l.Containers {
		if !slices.ContainsFunc(c.Volumes, func(v struct {
			DeviceIdentifier string   `json:"DeviceIdentifier"`
			Name             string   `json:"Name"`
			Roles            []string `json:"Roles"`
			CapacityInUse    uint64   `json:"CapacityInUse"`
		}) bool {
			return v.DeviceIdentifier == dev
		}) {
			continue
		}
		b.Capacity, b.Free = c.CapacityCeiling, c.CapacityFree
		b.Used = c.CapacityCeiling - min(c.CapacityFree, c.CapacityCeiling)
		b.Others = b.Others[:0]
		for _, v := range c.Volumes {
			role := ""
			if len(v.Roles) > 0 {
				role = v.Roles[0]
			}
			switch {
			case v.DeviceIdentifier == dev:
				b.DataUsed = v.CapacityInUse
			case role == "System":
				b.SystemUsed = v.CapacityInUse
			default:
				b.Others = append(b.Others, VolumeUse{Name: v.Name, Role: role, Device: v.DeviceIdentifier, Used: v.CapacityInUse})
			}
		}
		return nil
	}
	return fmt.Errorf("no APFS container holds %s", dev)
}
