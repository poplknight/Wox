package info

import (
	"net"
	"net/netip"
	"os"
	"os/user"
	"strings"
)

// readHostIdentity fills the cross-platform user, hostname, and address fields.
func readHostIdentity(snap *infoSnapshot) {
	if snap.Hostname == "" {
		if hostname, err := os.Hostname(); err == nil {
			snap.Hostname = hostname
		}
	}
	if snap.UserName == "" {
		current, err := user.Current()
		if err == nil {
			snap.UserName = current.Username
			snap.FullName = current.Name
			snap.Home = current.HomeDir
		}
	}
	snap.Addresses = readAddresses()
}

// readAddresses lists addresses on interfaces that are currently up.
func readAddresses() []nicAddress {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	addresses := make([]nicAddress, 0, len(ifaces))
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		nic := nicAddress{Name: iface.Name}
		for _, addr := range addrs {
			ip := addressIP(addr)
			if !ip.IsValid() || ip.IsLoopback() || ip.IsUnspecified() {
				continue
			}
			switch {
			case ip.Is4():
				nic.IPv4 = append(nic.IPv4, ip.String())
			case ip.IsLinkLocalUnicast():
				nic.LinkLocal = append(nic.LinkLocal, ip.String())
			default:
				nic.IPv6 = append(nic.IPv6, ip.String())
			}
		}
		if len(nic.IPv4)+len(nic.IPv6)+len(nic.LinkLocal) == 0 {
			continue
		}
		addresses = append(addresses, nic)
	}
	return addresses
}

func addressIP(addr net.Addr) netip.Addr {
	switch value := addr.(type) {
	case *net.IPNet:
		ip, _ := netip.AddrFromSlice(value.IP)
		return ip.Unmap()
	case *net.IPAddr:
		ip, _ := netip.AddrFromSlice(value.IP)
		return ip.Unmap()
	default:
		ip, err := netip.ParseAddr(strings.Split(addr.String(), "/")[0])
		if err != nil {
			return netip.Addr{}
		}
		return ip.Unmap()
	}
}
