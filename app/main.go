package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
)

const usageText = `Usage:
  transcribe-vosk -input <path> -sample-rate <rate> -server <ws://host:port>
  transcribe-vosk <path_to_audio_file> <sample_rate> <hostname> <port>
`

type cliConfig struct {
	inputFile  string
	sampleRate int
	serverURL  string
}

type voskResponse struct {
	Text    *string `json:"text"`
	Partial *string `json:"partial"`
}

func main() {
	log.SetFlags(0)

	if err := run(os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(args []string, stdout io.Writer) error {
	cfg, err := parseArgs(args)
	if err != nil {
		return err
	}

	audioData, err := convertAudio(cfg.inputFile, cfg.sampleRate)
	if err != nil {
		return err
	}

	transcription, err := transcribe(cfg.serverURL, cfg.sampleRate, audioData)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(stdout, transcription)
	return err
}

func parseArgs(args []string) (cliConfig, error) {
	if len(args) == 4 && !strings.HasPrefix(args[0], "-") {
		sampleRate, err := strconv.Atoi(args[1])
		if err != nil {
			return cliConfig{}, fmt.Errorf("invalid sample rate %q: %w", args[1], err)
		}

		cfg := cliConfig{
			inputFile:  args[0],
			sampleRate: sampleRate,
			serverURL:  fmt.Sprintf("ws://%s:%s", args[2], args[3]),
		}

		return cfg, validateConfig(cfg)
	}

	var cfg cliConfig

	fs := flag.NewFlagSet("transcribe-vosk", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.inputFile, "input", "", "path to input audio file")
	fs.IntVar(&cfg.sampleRate, "sample-rate", 0, "audio sample rate to send to Vosk")
	fs.StringVar(&cfg.serverURL, "server", "ws://localhost:2700", "Vosk websocket URL")

	if err := fs.Parse(args); err != nil {
		return cliConfig{}, fmt.Errorf("%w\n\n%s", err, usageText)
	}

	if len(fs.Args()) != 0 {
		return cliConfig{}, fmt.Errorf("unexpected positional arguments: %s\n\n%s", strings.Join(fs.Args(), " "), usageText)
	}

	if err := validateConfig(cfg); err != nil {
		return cliConfig{}, err
	}

	return cfg, nil
}

func validateConfig(cfg cliConfig) error {
	if cfg.inputFile == "" {
		return errors.New("missing input audio file\n\n" + usageText)
	}

	if cfg.sampleRate <= 0 {
		return errors.New("sample rate must be greater than zero\n\n" + usageText)
	}

	serverURL, err := url.Parse(cfg.serverURL)
	if err != nil {
		return fmt.Errorf("invalid server URL %q: %w", cfg.serverURL, err)
	}
	if serverURL.Host == "" || (serverURL.Scheme != "ws" && serverURL.Scheme != "wss") {
		return fmt.Errorf("server URL must use ws:// or wss:// and include a host: %q", cfg.serverURL)
	}

	return nil
}

func convertAudio(inputFile string, sampleRate int) ([]byte, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, errors.New("ffmpeg is required in PATH")
	}

	cmd := exec.Command(
		"ffmpeg",
		"-v", "error",
		"-i", inputFile,
		"-ar", strconv.Itoa(sampleRate),
		"-ac", "1",
		"-f", "s16le",
		"-",
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("ffmpeg conversion failed: %s", strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("ffmpeg conversion failed: %w", err)
	}

	return stdout.Bytes(), nil
}

func transcribe(serverURL string, sampleRate int, audioData []byte) (string, error) {
	conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to connect to Vosk server: %w", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]any{
		"config": map[string]int{
			"sample_rate": sampleRate,
		},
	}); err != nil {
		return "", fmt.Errorf("failed to send config: %w", err)
	}

	chunkSize := sampleRate / 5 * 2
	if chunkSize <= 0 {
		chunkSize = 3200
	}

	for i := 0; i < len(audioData); i += chunkSize {
		end := i + chunkSize
		if end > len(audioData) {
			end = len(audioData)
		}

		if err := conn.WriteMessage(websocket.BinaryMessage, audioData[i:end]); err != nil {
			return "", fmt.Errorf("failed to write audio chunk: %w", err)
		}

		if _, err := readResponse(conn); err != nil {
			return "", err
		}
	}

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"eof" : 1}`)); err != nil {
		return "", fmt.Errorf("failed to send eof: %w", err)
	}

	for {
		response, err := readResponse(conn)
		if err != nil {
			return "", err
		}

		if response.Text != nil {
			return *response.Text, nil
		}
	}
}

func readResponse(conn *websocket.Conn) (voskResponse, error) {
	_, message, err := conn.ReadMessage()
	if err != nil {
		return voskResponse{}, fmt.Errorf("failed to receive response: %w", err)
	}

	var response voskResponse
	if err := json.Unmarshal(message, &response); err != nil {
		return voskResponse{}, fmt.Errorf("failed to parse response %q: %w", string(message), err)
	}

	return response, nil
}
