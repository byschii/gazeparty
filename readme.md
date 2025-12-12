# GAZEPARTY

Streaming video server che esplora cartelle e riproduce file video dinamicamente.

## Stack tecnico

- **Backend**: Go + Gin framework
- **Template**: Go templates (html/template)
- **Containerizzazione**: Docker + Docker Compose
- **Dipendenze**: ffmpeg/ffprobe (installati nel container)

## Architettura

**`/<path>`** → Esplora cartelle o mostra player (a seconda se è dir o file)
**`/stream/<path>`** → Stream video binario

## Flusso utente

1. Apri `/` → vedi le cartelle/file della root
2. Click su una cartella → `/movies` → lista aggiornata
3. Click su un video → `/movies/film.mkv` → player con streaming automatico
4. Puoi condividere il link diretto `/movies/film.mkv` con un amico

---

## Avvio

### Con Docker Compose

```bash
# Modifica il path nel docker-compose.yml
docker-compose up -d
```

Accedi a `http://localhost:8080`

### Sviluppo locale

```bash
go mod download
go run main.go
```

---

## Struttura progetto

```
.
├── main.go              # Handler principale e routing
├── Dockerfile           # Build multi-stage con ffmpeg
├── docker-compose.yml   # Compose per containerizzazione
├── go.mod / go.sum      # Dipendenze Go
└── templates/
    ├── browser.html     # Pagina esplorazione cartelle
    └── player.html      # Pagina player video
```

---

## Note implementative

- **Navigazione dinamica**: ogni richiesta legge il filesystem live
- **Sicurezza**: sanitizzazione path per prevenire traversal attacks
- **Transcode on-demand**: ffmpeg converte codec non supportati
- **Ottimizzazioni Raspberry Pi**: preset ultrafast + CRF 23 + audio 128k
