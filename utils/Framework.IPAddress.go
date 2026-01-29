package utils

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	qrTerminal "github.com/mdp/qrterminal/v3"
)

// GetIPs returns a slice of IP addresses of the local machine.
// If includeLocalhost is true, it also includes "localhost" in the result.
// It filters out loopback addresses and only includes IPv4 addresses.
// Note: This function does not handle errors from net.InterfaceAddrs().
// It is assumed that the caller will handle any potential errors.
// The function is useful for logging or displaying server addresses.
func GetIPs(includeLocalhost bool) []string {
	results := make([]string, 0)

	addrs, _ := net.InterfaceAddrs()

	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				results = append(results, ipnet.IP.String())
			}
		}
	}
	if includeLocalhost {
		results = append(results, "localhost")
	}
	sort.Strings(results)
	return results
}

func serverURLs(https bool, port int, includeLocalhost bool) []string {
	protocol := "http://"
	if https {
		protocol = "https://"
	}
	portPart := fmt.Sprint(port)
	ips := GetIPs(includeLocalhost)
	urls := make([]string, 0, len(ips))
	for _, ip := range ips {
		urls = append(urls, protocol+ip+":"+portPart)
	}
	return urls
}

// LogServer prints the server address in a formatted way.
// It takes a boolean indicating whether to use HTTPS and an integer for the port.
func LogServer(https bool, port int) {
	ensureColorOutput()
	fmt.Println("Server available:")
	for _, href := range serverURLs(https, port, true) {
		color.New(color.FgGreen, color.BgBlue).Println(href)
	}
}

// LogServerWithQR prints the server address and renders a QR code in terminal.
// It will prioritize the first non-localhost URL for QR, fallback to localhost.
func LogServerWithQR(https bool, port int, includeLocalhost bool) {
	ensureColorOutput()
	urls := serverURLs(https, port, includeLocalhost)
	if len(urls) == 0 {
		fmt.Println("Server available: none")
		return
	}
	fmt.Println("Server available:")
	for _, href := range urls {
		color.New(color.FgGreen, color.BgBlue).Println(href)
	}

	qrURL := urls[0]
	for _, href := range urls {
		if strings.HasPrefix(href, "http://localhost:") || strings.HasPrefix(href, "https://localhost:") {
			continue
		}
		qrURL = href
		break
	}
	fmt.Println("Scan QR to open:")
	qrTerminal.GenerateHalfBlock(qrURL, qrTerminal.M, os.Stdout)
}

func ensureColorOutput() {
	stdoutTTY := isatty.IsTerminal(os.Stdout.Fd())
	stderrTTY := isatty.IsTerminal(os.Stderr.Fd())

	if !stdoutTTY && stderrTTY {
		color.Output = os.Stderr
		color.NoColor = false
	}
}
