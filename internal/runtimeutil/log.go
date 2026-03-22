package runtimeutil

import (
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	qrTerminal "github.com/mdp/qrterminal/v3"
)

var goLogPrefix = color.New(color.FgGreen).Sprint("[go]")

func Print(v ...any) {
	if len(v) == 0 {
		fmt.Println(goLogPrefix)
		return
	}
	values := append([]any{goLogPrefix}, v...)
	fmt.Println(values...)
}

type ServerLogStyle string

const (
	ServerLogStyleNeon   ServerLogStyle = "neon"
	ServerLogStyleSunset ServerLogStyle = "sunset"
	ServerLogStyleOcean  ServerLogStyle = "ocean"
	ServerLogStyleForest ServerLogStyle = "forest"
	ServerLogStyleMono   ServerLogStyle = "mono"
)

var currentServerLogStyle = ServerLogStyleOcean

func SetServerLogStyle(style ServerLogStyle) {
	switch style {
	case ServerLogStyleSunset:
		currentServerLogStyle = ServerLogStyleSunset
	case ServerLogStyleOcean:
		currentServerLogStyle = ServerLogStyleOcean
	case ServerLogStyleForest:
		currentServerLogStyle = ServerLogStyleForest
	case ServerLogStyleMono:
		currentServerLogStyle = ServerLogStyleMono
	default:
		currentServerLogStyle = ServerLogStyleNeon
	}
}

func ServerLogStyles() []ServerLogStyle {
	return []ServerLogStyle{
		ServerLogStyleNeon,
		ServerLogStyleSunset,
		ServerLogStyleOcean,
		ServerLogStyleForest,
		ServerLogStyleMono,
	}
}

func GetIPs(includeLocalhost bool) []string {
	results := make([]string, 0)
	addrs, _ := net.InterfaceAddrs()
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			results = append(results, ipnet.IP.String())
		}
	}
	if includeLocalhost {
		results = append(results, "localhost")
	}
	sort.Strings(results)
	return results
}

func LogServerWithBasePath(https bool, port int, basePath string) {
	ensureColorOutput()
	urls := serverURLs(https, port, true, basePath)
	printServerURLs(urls, false)
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
		Print(localLabel + hintColor.Sprint("none"))
	} else {
		Print(localLabel + urlColor.Sprint(localURL))
	}

	if len(networkURLs) == 0 {
		Print(networkLabel + hintColor.Sprint("none"))
		return
	}
	for i, href := range networkURLs {
		line := networkLabel + urlColor.Sprint(href)
		if withQRHint && i == 0 {
			line += hintColor.Sprint(" [QR code]")
		}
		Print(line)
	}
}

func serverLogPalette() (localLabel string, networkLabel string, urlColor *color.Color, hintColor *color.Color) {
	switch currentServerLogStyle {
	case ServerLogStyleSunset:
		return color.New(color.FgHiYellow).Sprint("➜ Local 本地:    "),
			color.New(color.FgHiRed).Sprint("➜ Network 局域网: "),
			color.New(color.FgHiWhite),
			color.New(color.FgHiBlack)
	case ServerLogStyleOcean:
		return color.New(color.FgHiCyan).Sprint("➜ Local 本地:    "),
			color.New(color.FgBlue).Sprint("➜ Network 局域网: "),
			color.New(color.FgHiBlue),
			color.New(color.FgHiBlack)
	case ServerLogStyleForest:
		return color.New(color.FgGreen).Sprint("➜ Local 本地:    "),
			color.New(color.FgHiGreen).Sprint("➜ Network 局域网: "),
			color.New(color.FgHiYellow),
			color.New(color.FgHiBlack)
	case ServerLogStyleMono:
		return color.New(color.FgWhite).Sprint("➜ Local 本地:    "),
			color.New(color.FgHiBlack).Sprint("➜ Network 局域网: "),
			color.New(color.FgWhite),
			color.New(color.FgHiBlack)
	default:
		return color.New(color.FgHiGreen).Sprint("➜ Local 本地:    "),
			color.New(color.FgMagenta).Sprint("➜ Network 局域网: "),
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

func renderQRCode(url string) {
	out := qrOutputWriter()
	_, _ = fmt.Fprintln(out)
	qrTerminal.GenerateHalfBlock(url, qrTerminal.M, out)
	_, _ = fmt.Fprintf(out, "\n%s\n", url)
}

func qrOutputWriter() io.Writer {
	if color.Output == os.Stderr {
		return os.Stderr
	}
	return os.Stdout
}
