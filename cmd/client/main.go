// client/main.go
package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/heysubinoy/ngopen/protocol"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/xtaci/smux"
)

var (
	cfgFile string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "ngopen",
		Short: "Expose your local service to the internet via a secure tunnel",
		Run:   runClient,
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.ngopen/config.yaml)")
	rootCmd.PersistentFlags().String("hostname", "AUTO", "Subdomain to register or 'AUTO' to let server generate one")
	rootCmd.PersistentFlags().String("local", "", "Local service to forward to")
	rootCmd.PersistentFlags().String("server", "tunnel.n.sbn.lol:9000", "Tunnel server address")
	rootCmd.PersistentFlags().Duration("reconnect-delay", 5*time.Second, "Delay between reconnection attempts")
	rootCmd.PersistentFlags().Bool("preserve-ip", true, "Preserve original client IP in X-Forwarded-For header")
	rootCmd.PersistentFlags().String("auth", "", "Authentication token for server")

	viper.BindPFlag("hostname", rootCmd.PersistentFlags().Lookup("hostname"))
	viper.BindPFlag("local", rootCmd.PersistentFlags().Lookup("local"))
	viper.BindPFlag("server", rootCmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("reconnect-delay", rootCmd.PersistentFlags().Lookup("reconnect-delay"))
	viper.BindPFlag("preserve-ip", rootCmd.PersistentFlags().Lookup("preserve-ip"))
	viper.BindPFlag("auth", rootCmd.PersistentFlags().Lookup("auth"))

	cobra.OnInitialize(initConfig)

	// Config subcommand
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage persistent ngopen config",
	}
	configSetCmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			key, value := args[0], args[1]
			viper.Set(key, value)
			if err := viper.WriteConfigAs(configPath()); err != nil {
				color.Red("❌ Failed to write config: %v", err)
				os.Exit(1)
			}
			color.Green("✓ Set %s = %s", key, value)
		},
	}
	configGetCmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			val := viper.GetString(key)
			fmt.Printf("%s = %s\n", key, val)
		},
	}
	configListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all config values",
		Run: func(cmd *cobra.Command, args []string) {
			all := viper.AllSettings()
			for k, v := range all {
				fmt.Printf("%s = %v\n", k, v)
			}
		},
	}
	configCmd.AddCommand(configSetCmd, configGetCmd, configListCmd)
	rootCmd.AddCommand(configCmd)

	if err := rootCmd.Execute(); err != nil {
		color.Red("❌ %v", err)
		os.Exit(1)
	}
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			color.Red("❌ Unable to find home directory: %v", err)
			os.Exit(1)
		}
		configDir := home + string(os.PathSeparator) + ".ngopen"
		os.MkdirAll(configDir, 0700)
		viper.AddConfigPath(configDir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}
	viper.SetEnvPrefix("NGOPEN")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err == nil {
		color.Cyan("Using config file: %s", viper.ConfigFileUsed())
	}
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		color.Red("❌ Unable to find home directory: %v", err)
		os.Exit(1)
	}
	return home + string(os.PathSeparator) + ".ngopen" + string(os.PathSeparator) + "config.yaml"
}

func runClient(cmd *cobra.Command, args []string) {
	hostname := viper.GetString("hostname")
	local := viper.GetString("local")
	server := viper.GetString("server")
	reconnectDelay := viper.GetDuration("reconnect-delay")
	preserveClientIP := viper.GetBool("preserve-ip")
	authToken := viper.GetString("auth")

	// If no flags or arguments are provided, show usage and return
	if len(os.Args) == 1 || (hostname == "AUTO" && local == "" && server == "tunnel.n.sbn.lol:9000" && authToken == "") {
		cmd.Help()
		return
	}

	if hostname == "" || local == "" {
		cmd.Help()
		return
	}
	if authToken == "" {
		cmd.Help()
		return
	}

	// Setup graceful shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	stop := make(chan struct{})

	go func() {
		<-signals
		logInfo("Shutting down client (signal received)...")
		close(stop)
		time.Sleep(250 * time.Millisecond)
		os.Exit(0)
	}()

	logInfo("Client starting up...")
	lastAssignedHostname := hostname

	firstAttempt := true
	for {
		select {
		case <-stop:
			return
		default:
			assignedHostname, err := connectAndServe(lastAssignedHostname, local, server, preserveClientIP, authToken)
			if err != nil {
				if firstAttempt {
					logError("Initial connection/authentication failed: %v. Not retrying.", err)
					return
				}
				if assignedHostname != "" {
					lastAssignedHostname = assignedHostname
				}
				logError("Connection error: %v. Reconnecting to %s in %v...", err, lastAssignedHostname, reconnectDelay)
				select {
				case <-stop:
					return
				case <-time.After(reconnectDelay):
				}
			} else if assignedHostname != "" {
				firstAttempt = false
				lastAssignedHostname = assignedHostname
				logInfo("Server closed connection for hostname '%s'. Reconnecting...", lastAssignedHostname)
				select {
				case <-stop:
					return
				case <-time.After(reconnectDelay):
				}
			} else {
				firstAttempt = false
			}
		}
	}
}

// --- Logging helpers ---
func logSuccess(format string, v ...interface{}) {
	args := []interface{}{color.New(color.FgGreen).Sprint(time.Now().Format("2006-01-02 15:04:05")), "✓"}
	args = append(args, v...)
	log.Printf("%s %s "+format, args...)
}

func logInfo(format string, v ...interface{}) {
	args := []interface{}{color.New(color.FgCyan).Sprint(time.Now().Format("2006-01-02 15:04:05")), "ℹ️"}
	args = append(args, v...)
	log.Printf("%s %s "+format, args...)
}

func logError(format string, v ...interface{}) {
	args := []interface{}{color.New(color.FgRed).Sprint(time.Now().Format("2006-01-02 15:04:05")), "❌"}
	args = append(args, v...)
	log.Printf("%s %s "+format, args...)
}

// --- Main tunnel logic (unchanged) ---
func connectAndServe(hostname, local, server string, preserveClientIP bool, authToken string) (string, error) {
	logInfo("Attempting to connect to server at %s", server)
	conn, err := net.Dial("tcp", server)
	if err != nil {
		logError("TCP connection to %s failed: %v", server, err)
		return "", fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		logInfo("TCP connection to %s closed", server)
		conn.Close()
	}()

	logInfo("Connected to server at %s. Registering with hostname request: '%s'", server, hostname)
	conn.SetDeadline(time.Time{})

	logInfo("Establishing smux session...")
	session, err := smux.Client(conn, nil)
	if err != nil {
		logError("Failed to create smux session: %v", err)
		return hostname, fmt.Errorf("failed to create smux session: %w", err)
	}
	defer func() {
		logInfo("smux session closed")
		session.Close()
	}()

	logInfo("Opening first stream for authentication...")
	authStream, err := session.OpenStream()
	if err != nil {
		logError("Failed to open auth stream: %v", err)
		return hostname, fmt.Errorf("failed to open auth stream: %w", err)
	}

	authMsg := protocol.ProtocolAuthMessage{
		AuthToken: authToken,
		Hostname:  hostname,
	}
	encoded, err := protocol.EncodeProtocolAuthMessage(authMsg)
	if err != nil {
		logError("Failed to encode auth message: %v", err)
		authStream.Close()
		return hostname, fmt.Errorf("failed to encode auth message: %w", err)
	}
	if _, err := authStream.Write(encoded); err != nil {
		logError("Failed to send auth message: %v", err)
		authStream.Close()
		return hostname, fmt.Errorf("failed to send auth message: %w", err)
	}

	respHeader := make([]byte, 4)
	if _, err := io.ReadFull(authStream, respHeader); err != nil {
		logError("Failed to read auth response header: %v", err)
		authStream.Close()
		return hostname, fmt.Errorf("failed to read auth response header: %w", err)
	}
	respLen := binary.BigEndian.Uint32(respHeader)
	respPayload := make([]byte, respLen)
	if _, err := io.ReadFull(authStream, respPayload); err != nil {
		logError("Failed to read auth response payload: %v", err)
		authStream.Close()
		return hostname, fmt.Errorf("failed to read auth response payload: %w", err)
	}
	authStream.Close()

	respStr := string(respPayload)
	if len(respStr) >= 3 && respStr[:3] == "OK:" {
		assignedHostname := respStr[3:]
		logSuccess("Authenticated. Assigned hostname: %s", assignedHostname)
		fmt.Println()
		color.Green("✓ Tunnel established")
		fmt.Printf("%s https://%s %s %s\n",
			color.GreenString("✓ Forwarding"),
			assignedHostname,
			color.GreenString("->"),
			color.CyanString(local),
		)
		color.Green("✓ Ready for connections")
		logInfo("Tunnel established and ready for connections on https://%s", assignedHostname)
		hostname = assignedHostname
	} else {
		logError("Authentication failed: %s", respStr)
		color.Red("❌ Authentication failed: %s", respStr)
		return hostname, fmt.Errorf("authentication failed: %s", respStr)
	}

	for {
		stream, err := session.AcceptStream()
		if err != nil {
			logError("Failed to accept stream: %v", err)
			return hostname, fmt.Errorf("failed to accept stream: %w", err)
		}
		logInfo("Accepted new stream from server. Handling HTTP request...")
		go handleStream(stream, local, preserveClientIP)
	}
}

func handleStream(stream net.Conn, local string, preserveClientIP bool) {
	defer func() {
		logInfo("Closed stream for local service %s", local)
		stream.Close()
	}()

	header := make([]byte, 4)
	if _, err := io.ReadFull(stream, header); err != nil {
		logError("Error reading stream header: %v", err)
		return
	}
	reqLen := binary.BigEndian.Uint32(header)
	reqBytes := make([]byte, reqLen)
	if _, err := io.ReadFull(stream, reqBytes); err != nil {
		logError("Error reading stream request: %v", err)
		return
	}

	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(reqBytes)))
	if err != nil {
		logError("Error parsing HTTP request: %v", err)
		return
	}

	clientIP := req.Header.Get("X-Forwarded-For")
	remoteAddrStr := req.RemoteAddr
	logInfo("Handling HTTP request for %s (client IP: %s, remote: %s)", req.URL.Path, clientIP, remoteAddrStr)
	req.RequestURI = ""
	req.URL.Scheme = "http"
	req.URL.Host = local

	if preserveClientIP && clientIP != "" {
		logInfo("Preserving client IP: %s", clientIP)
	}

	if !strings.Contains(req.URL.Path, "/_next/webpack-hmr") {
		sourceIP := clientIP
		if sourceIP == "" {
			sourceIP = remoteAddrStr
		}
		logSuccess("Request: %s %s (from %s)", req.Method, req.URL.Path, sourceIP)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logError("Local forward failed: %v", err)
		resp = &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader("Failed to forward to local service")),
			Header:     make(http.Header),
			ProtoMajor: 1,
			ProtoMinor: 1,
		}
	} else {
		if !strings.Contains(req.URL.Path, "/_next/webpack-hmr") {
			logSuccess("Response: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		}
	}

	var buf bytes.Buffer
	if err := resp.Write(&buf); err != nil {
		logError("Error encoding response: %v", err)
		return
	}
	respBytes := buf.Bytes()
	respLen := uint32(len(respBytes))
	lengthHeader := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthHeader, respLen)

	stream.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := stream.Write(append(lengthHeader, respBytes...)); err != nil {
		logError("Error sending response on stream: %v", err)
		return
	}
	stream.SetWriteDeadline(time.Time{})
}
