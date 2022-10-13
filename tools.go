package discovery

import "net"

func FilterInterfaces(ipv4 bool) (ifaces []*net.Interface, err error) {
	allIfaces, err := net.Interfaces()
	if err != nil {
		return
	}
	ifaces = make([]*net.Interface, 0, len(allIfaces))
	for i := range allIfaces {
		iface := allIfaces[i]
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagBroadcast == 0 {
			// interface is down or does not support broadcasting
			continue
		}
		addrs, _ := iface.Addrs()
		supported := false
		for j := range addrs {
			addr := addrs[j].(*net.IPNet)
			if addr == nil || addr.IP == nil {
				continue
			}
			isv4 := addr.IP.To4() != nil
			if isv4 == ipv4 {
				// IP family matches, go on and use interface
				supported = true
				break
			}
		}
		if supported {
			ifaces = append(ifaces, &iface)
		}
	}
	return
}

func GetLocalIPs() (ips map[string]struct{}) {
	ips = make(map[string]struct{})
	ips["localhost"] = struct{}{}
	ips["127.0.0.1"] = struct{}{}
	ips["::1"] = struct{}{}

	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, address := range addrs {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil {
				// log.Printf("Failed to parse %s: %v", address.String(), err)
				continue
			}

			ips[ip.String()+"%"+iface.Name] = struct{}{}
			ips[ip.String()] = struct{}{}
		}
	}
	return
}
