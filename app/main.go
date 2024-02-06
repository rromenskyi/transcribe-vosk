package main

import (
	"encoding/json"
	"fmt"
_	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"

	"github.com/gorilla/websocket"
)

type VoskResponse struct {
	Text string `json:"text"`
}

func main() {
	// Проверка аргументов командной строки
	if len(os.Args) < 5 {
		log.Fatal("Usage: go run main.go <path_to_audio_file> <sample_rate> <hostname> <port>")
	}

	inputFile := os.Args[1]

	sampleRateStr := os.Args[2]
	sampleRate, err := strconv.Atoi(sampleRateStr)
	if err != nil {
		log.Fatal("Invalid sample rate:", err)
	}

	hostname := os.Args[3]
	port := os.Args[4]
	serverURL := fmt.Sprintf("ws://%s:%s", hostname, port)

	// Преобразование аудио с помощью FFmpeg
	cmd := exec.Command("ffmpeg", "-i", inputFile, "-ar", fmt.Sprintf("%d", sampleRate), "-ac", "1", "-f", "s16le", "-")
	audioData, err := cmd.Output()
	if err != nil {
		log.Fatal("Failed to process audio:", err)
	}

	conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
	if err != nil {
		log.Fatal("Failed to connect to Vosk server:", err)
	}
	defer conn.Close()

	const chunkSize = 1048576

	// Чтение и отправка аудиофайла по частям
	for i := 0; i < len(audioData); i += chunkSize {
	    end := i + chunkSize
	    if end > len(audioData) {
	        end = len(audioData)
	    }
	    chunk := audioData[i:end]
	    err := conn.WriteMessage(websocket.BinaryMessage, chunk)
	    if err != nil {
	        log.Fatal("Failed to write audio chunk:", err)
	    }
	}

	// Получение результата распознавания
	_, message, err := conn.ReadMessage()
	if err != nil {
		log.Fatal("Failed to receive response:", err)
	}

	var response VoskResponse
	err = json.Unmarshal(message, &response)
	if err != nil {
		log.Fatal("Failed to parse response:", err)
	}

	fmt.Println("Transcription:", response.Text)
}

