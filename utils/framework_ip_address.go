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

// ServerLogStyle controls the color theme of LogServer output.
type ServerLogStyle string

const (
	// ServerLogStyleNeon keeps the current bright green/magenta/cyan style.
	ServerLogStyleNeon ServerLogStyle = "neon"
	// ServerLogStyleSunset uses a warm orange/yellow palette.
	ServerLogStyleSunset ServerLogStyle = "sunset"
)

var currentServerLogStyle = ServerLogStyleNeon

// SetServerLogStyle changes LogServer output color theme.
func SetServerLogStyle(style ServerLogStyle) {
	switch style {
	case ServerLogStyleSunset:
		currentServerLogStyle = ServerLogStyleSunset
	default:
		currentServerLogStyle = ServerLogStyleNeon
	}
}

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

func serverURLs(https bool, port int, includeLocalhost bool, basePath string) []string {
	protocol := "http://"
	if https {
		protocol = "https://"
	}
	portPart := fmt.Sprint(port)
	pathPart := normalizeBasePath(basePath)
	ips := GetIPs(includeLocalhost)
	urls := make([]string, 0, len(ips))
	for _, ip := range ips {
		urls = append(urls, protocol+ip+":"+portPart+pathPart)
	}
	return urls
}

// LogServer prints the server address in a formatted way.
// It takes a boolean indicating whether to use HTTPS and an integer for the port.
func LogServer(https bool, port int) {
	LogServerWithBasePath(https, port, "/")
}

// LogServerWithBasePath prints Local/Network URLs with a styled format.
func LogServerWithBasePath(https bool, port int, basePath string) {
	ensureColorOutput()
	urls := serverURLs(https, port, true, basePath)
	printServerURLs(urls, false)
}

// LogServerWithQR prints the server address and renders a QR code in terminal.
// It will prioritize the first non-localhost URL for QR, fallback to localhost.
func LogServerWithQR(https bool, port int, includeLocalhost bool) {
	LogServerWithQRAndBasePath(https, port, includeLocalhost, "/")
}

// LogServerWithQRAndBasePath prints Local/Network URLs and renders QR code.
func LogServerWithQRAndBasePath(https bool, port int, includeLocalhost bool, basePath string) {
	ensureColorOutput()
	urls := serverURLs(https, port, includeLocalhost, basePath)
	if len(urls) == 0 {
		printServerURLs(urls, true)
		return
	}
	printServerURLs(urls, true)

	qrURL := urls[0]
	for _, href := range urls {
		if strings.HasPrefix(href, "http://localhost:") || strings.HasPrefix(href, "https://localhost:") {
			continue
		}
		qrURL = href
		break
	}
	fmt.Println(color.New(color.FgHiBlack).Sprint("Scan QR to open:"))
	qrTerminal.GenerateHalfBlock(qrURL, qrTerminal.M, os.Stdout)
}

func printServerURLs(urls []string, withQRHint bool) {
	localLabel, networkLabel, urlColor, hintColor := serverLogPalette()

	localURL := ""
	networkURLs := make([]string, 0, len(urls))
	for _, href := range urls {
		if strings.Contains(href, "://localhost:") {
			localURL = href
			continue
		}
		networkURLs = append(networkURLs, href)
	}

	if localURL == "" {
		fmt.Println(localLabel + hintColor.Sprint("none"))
	} else {
		fmt.Println(localLabel + urlColor.Sprint(localURL))
	}

	if len(networkURLs) == 0 {
		fmt.Println(networkLabel + hintColor.Sprint("none"))
		return
	}
	for i, href := range networkURLs {
		line := networkLabel + urlColor.Sprint(href)
		if withQRHint && i == 0 {
			line += hintColor.Sprint(" [QR code]")
		}
		fmt.Println(line)
	}
}

func serverLogPalette() (localLabel string, networkLabel string, urlColor *color.Color, hintColor *color.Color) {
	switch currentServerLogStyle {
	case ServerLogStyleSunset:
		return color.New(color.FgHiYellow).Sprint("➜ Local:   "),
			color.New(color.FgHiRed).Sprint("➜ Network: "),
			color.New(color.FgHiWhite),
			color.New(color.FgHiBlack)
	default:
		return color.New(color.FgHiGreen).Sprint("➜ Local:   "),
			color.New(color.FgMagenta).Sprint("➜ Network: "),
			color.New(color.FgHiCyan),
			color.New(color.FgHiBlack)
	}
}

func normalizeBasePath(basePath string) string {
	path := strings.TrimSpace(basePath)
	if path == "" || path == "/" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(path, "/")
}

func ensureColorOutput() {
	stdoutTTY := isatty.IsTerminal(os.Stdout.Fd())
	stderrTTY := isatty.IsTerminal(os.Stderr.Fd())

	if !stdoutTTY && stderrTTY {
		color.Output = os.Stderr
		color.NoColor = false
	}
}
