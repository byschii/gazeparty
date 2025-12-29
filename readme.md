# GAZEPARTY

Streaming video server basato su HLS con segmentazione on-demand.

## Stack tecnico

- **Backend**: Go + Gin framework
- **Frontend**: HTML statico + hls.js
- **Streaming**: HLS (HTTP Live Streaming)
- **Containerizzazione**: Docker + Docker Compose
- **Dipendenze**: ffmpeg/ffprobe (installati nel container)

## Architettura

**`/files`** → API JSON con lista video
**`/stream/:id/playlist.m3u8`** → Playlist HLS
**`/stream/:id/segment_X.ts`** → Segmenti video generati on-demand

## Flusso utente

1. Apri `/static/index.html` → lista video caricata via API
2. Click su un video → `/static/player.html?id=...` → player HLS
3. Player richiede playlist → segmenti generati e cachati in `/tmp`
4. Puoi condividere il link diretto del player con l'ID video

---

## Avvio

### Con Docker Compose

```bash
# Inserisci i video in ./media
docker-compose up -d
```

Accedi a `http://localhost:8066/static/index.html`

### Sviluppo locale

```bash
go mod download
go run main.go
```

---

## Struttura progetto

```
.
├── main.go                # Routing principale
├── internal/
│   ├── handlers.go        # Gestione endpoints
│   ├── ffmpeg.go          # Generazione segmenti
│   └── utils.go           # Utility functions
├── static/
│   ├── index.html         # Lista video
│   └── player.html        # Player HLS
├── Dockerfile             # Build multi-stage con ffmpeg
├── docker-compose.yml     # Compose per containerizzazione
└── go.mod / go.sum        # Dipendenze Go
```

---

## Note implementative

- **HLS streaming**: segmentazione dinamica da 4 secondi
- **Generazione on-demand**: segmenti creati solo quando richiesti
- **Cache locale**: segmenti salvati in `/tmp/segments/:id/`
- **Transcode ottimizzato**: preset ultrafast + audio stereo 128k
