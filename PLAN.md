# crom-mediastream: Implementation Plan

`crom-mediastream` is a Go-based CLI/TUI application designed to stream a folder of MP4 files to platforms like Twitch and YouTube with professional fade transitions and real-time control.

## 🚀 Vision
A lightweight, powerful "TV Station in a box" that allows users to manage a 24/7 stream directly from their terminal.

---

## 🛠 Tech Stack
- **Language:** Go (Golang)
- **Video Engine:** FFmpeg (via `os/exec` and complex filters)
- **TUI Framework:** [Bubbletea](https://github.com/charmbracelet/bubbletea) (Charm.sh)
- **Styling:** [Lipgloss](https://github.com/charmbracelet/lipgloss)
- **Configuration:** [Viper](https://github.com/spf13/viper)
- **Streaming:** RTMP protocol (Twitch/YouTube)

---

## 🏗 System Architecture

### 1. File Watcher & Queue Manager
- Scans a target directory for `.mp4` files.
- Maintains a playlist/queue.
- Supports "Manual Override" to jump to a specific video.

### 2. FFmpeg Stream Engine
- **Core logic:** Spawns FFmpeg as a subprocess.
- **Transition Logic:** Uses `xfade` filters for smooth video transitions.
- **Protocol:** Pushes to `rtmp://...` ingest points.
- **Overlays:** Ability to dynamically update the stream source without dropping the connection (using a named pipe or FFmpeg's `concat` demuxer).

### 3. TUI Dashboard (The "Monitor")
- **Live Status:** Shows currently playing file, elapsed time, and bitrate.
- **File Browser:** List of available MP4s in the folder.
- **Controls:** 
  - `Enter`: Trigger immediate transition to selected video.
  - `S`: Stop stream.
  - `R`: Refresh folder.
- **Logs:** Real-time FFmpeg output monitoring.

---

## 📝 Development Roadmap

### Phase 1: Foundation (The Engine)
- [ ] Setup Go project structure.
- [ ] Implement basic FFmpeg wrapper to stream a single MP4 to an RTMP URL.
- [ ] Research `ffmpeg concat` and `xfade` for seamless folder-to-stream logic.

### Phase 2: The Monitor (TUI)
- [ ] Implement Bubbletea base model.
- [ ] Create a "File List" component using `bubbles/list`.
- [ ] Create a "Status Bar" showing stream health and current track.

### Phase 3: Transitions & Logic
- [ ] Implement the "Fade Transition" logic (handling audio and video synchronization).
- [ ] Add folder watching to automatically update the queue when files are added/removed.

### Phase 4: Integration
- [ ] Add support for Twitch/YouTube API for stream title updates.
- [ ] Config management for Stream Keys.

### Phase 5: Polish
- [ ] Handle network interruptions (auto-reconnect).
- [ ] Optimization of CPU usage during encoding.

---

## 🛠 Proposed Directory Structure
```text
crom-mediastream/
├── cmd/
│   └── crom/          # Main entry point
├── internal/
│   ├── engine/        # FFmpeg subprocess management
│   ├── queue/         # Playlist and folder scanning logic
│   ├── ui/            # Bubbletea components and views
│   └── config/        # Settings and API keys
├── assets/            # Default transitions or overlays
├── go.mod
└── PLAN.md
```

## 🎥 FFmpeg Command Concept (Fade)
```bash
ffmpeg -re -i current.mp4 -re -i next.mp4 \
-filter_complex "[0:v][1:v]xfade=transition=fade:duration=1:offset=10[v]" \
-map "[v]" -f flv rtmp://live.twitch.tv/app/{stream_key}
```
*(Note: Real implementation will likely use a continuous pipe to avoid stream drops between files.)*
