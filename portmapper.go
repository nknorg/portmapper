package portmapper

import (
	"errors"
	"fmt"
	"sync"
	"time"

	gonat "github.com/nknorg/go-nat"
	goupnp "gitlab.com/NebulousLabs/go-upnp"
)

var portMapDuration = map[string]time.Duration{"NAT-PMP": 365 * 86400 * time.Second}

type PortMapper struct {
	gateway     interface{}
	gatewayType string
	portsMapped sync.Map
}

func Discover() (*PortMapper, error) {
	pm := &PortMapper{}

	goupnpGateway, err := goupnp.Discover()
	if err == nil {
		pm.gateway = goupnpGateway
		pm.gatewayType = "UPnP"
	} else {
		gonatGateway, err := gonat.DiscoverGateway()
		if err == nil {
			pm.gateway = gonatGateway
			pm.gatewayType = gonatGateway.Type()
		} else {
			return nil, errors.New("no UPnP or NAT-PMP gateway found")
		}
	}

	return pm, nil
}

func (pm *PortMapper) Type() string {
	return pm.gatewayType
}

func (pm *PortMapper) ExternalIP() (string, error) {
	switch gateway := pm.gateway.(type) {
	case *goupnp.IGD:
		return gateway.ExternalIP()
	case gonat.NAT:
		addr, err := gateway.GetExternalAddress()
		return addr.String(), err
	default:
		return "", fmt.Errorf("unknown gateway type: %s", pm.Type())
	}
}

func (pm *PortMapper) Add(port uint16, description string) error {
	var err error
	switch gateway := pm.gateway.(type) {
	case *goupnp.IGD:
		err = gateway.Forward(port, description)
		if err != nil {
			return err
		}
	case gonat.NAT:
		_, _, err = gateway.AddPortMapping("tcp", int(port), int(port), description, portMapDuration[pm.Type()])
		if err != nil {
			return err
		}
		_, _, err = gateway.AddPortMapping("udp", int(port), int(port), description, portMapDuration[pm.Type()])
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown gateway type: %s", pm.Type())
	}
	pm.portsMapped.Store(port, description)
	return nil
}

func (pm *PortMapper) Delete(port uint16) error {
	var err error
	switch gateway := pm.gateway.(type) {
	case *goupnp.IGD:
		err = gateway.Clear(port)
		if err != nil {
			return err
		}
	case gonat.NAT:
		err = gateway.DeletePortMapping("tcp", int(port))
		if err != nil {
			return err
		}
		err = gateway.DeletePortMapping("udp", int(port))
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown gateway type: %s", pm.Type())
	}
	pm.portsMapped.Delete(port)
	return nil
}

func (pm *PortMapper) DeleteAll() error {
	var err error
	pm.portsMapped.Range(func(key interface{}, _ interface{}) bool {
		if port, ok := key.(uint16); ok {
			err = pm.Delete(port)
			if err != nil {
				return false
			}
		}
		return true
	})
	return err
}

func (pm *PortMapper) IsPortMapped(port uint16) bool {
	_, ok := pm.portsMapped.Load(port)
	return ok
}
