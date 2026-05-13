package controller

// UnprivilegedPort takes a port and lifts it into an unprivileged port range if it is privileged
// Source: https://en.wikipedia.org/wiki/List_of_TCP_and_UDP_port_numbers + https://www.iana.org/assignments/service-names-port-numbers/service-names-port-numbers.xml + https://www.w3.org/Daemon/User/Installation/PrivilegedPorts.html
func UnprivilegedPort[T int64 | int32 | int](given T) T {
	if given > 1024 {
		return given
	}
	if given < 100 {
		// Nice convention seen often,
		// e.g., 80 -> 8080
		//		443 -> 8443
		return given + 8000
	}
	return given + 1024
}
