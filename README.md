# transcribe-vosk

Minimal Go CLI for batch transcription through a Vosk WebSocket server.

It takes an audio file, converts it to mono 16-bit PCM with `ffmpeg`, streams
the audio to Vosk, and prints the final recognized text.

## What Is Vosk

Vosk is an offline speech-to-text toolkit. It can run locally or inside your
infrastructure without sending audio to a third-party cloud ASR provider.

This repository does not embed Vosk itself. It is a small client for a running
Vosk WebSocket server.

Official resources:

- Vosk project: <https://github.com/alphacep/vosk-api>
- Vosk server: <https://alphacephei.com/vosk/server>

## Why This Exists

The tool is useful when you already have recorded telephony audio, for example
from Asterisk, and want a simple command-line way to:

- normalize the source audio with `ffmpeg`
- send it to a Vosk server
- get back the final transcript

In short: Vosk does the speech recognition, this utility is the transport and
conversion glue around it.

## How It Works

1. Reads an input audio file supported by `ffmpeg`
2. Converts it to mono signed 16-bit PCM at the requested sample rate
3. Opens a WebSocket connection to the Vosk server
4. Sends Vosk a config message with the sample rate
5. Streams the audio in chunks
6. Sends EOF to finalize recognition
7. Prints the final transcript to stdout

## Requirements

- Go 1.22+
- `ffmpeg` available in `PATH`
- A reachable Vosk WebSocket server, for example `ws://localhost:2700`

## Quick Start

Start a Vosk server, for example:

```bash
docker run -d --rm -p 2700:2700 alphacep/kaldi-en:latest
```

Build the client:

```bash
go build -o bin/transcribe-vosk ./app
```

Run it:

```bash
./bin/transcribe-vosk \
  -input /path/to/audio.wav \
  -sample-rate 8000 \
  -server ws://localhost:2700
```

Example output:

```text
hello this is a test call
```

## Usage

Preferred flag-based form:

```bash
go run ./app \
  -input /path/to/audio.wav \
  -sample-rate 8000 \
  -server ws://localhost:2700
```

Legacy positional form is also supported:

```bash
go run ./app /path/to/audio.wav 8000 localhost 2700
```

Arguments:

- `-input`: input audio file path
- `-sample-rate`: sample rate expected by the server/model
- `-server`: full WebSocket URL, for example `ws://localhost:2700`

## Typical Telephony Use

For Asterisk-style recordings, `8000` is a common sample rate:

```bash
go run ./app \
  -input /var/spool/asterisk/monitor/call.wav \
  -sample-rate 8000 \
  -server ws://vosk.local:2700
```

## Notes

- The tool prints only the final transcript, not partial hypotheses.
- The audio format of the source file does not need to be raw PCM; `ffmpeg`
  handles the conversion step.
- Recognition quality depends mostly on the model, language, audio quality, and
  whether the sample rate matches what your setup expects.

## Repository Layout

```text
.
├── app/main.go
├── go.mod
├── LICENSE
└── README.md
```
