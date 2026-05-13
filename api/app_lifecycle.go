package handler

import (
	"log"
	"strings"
	"sync"
	"time"
)

const (
	appLifecycleHeartbeatTTL = 45 * time.Second
	appShutdownGrace         = 2 * time.Second
)

type appLifecycleState struct {
	ActiveClients int    `json:"activeClients"`
	Event         string `json:"event"`
	Shutdown      bool   `json:"shutdownScheduled"`
}

var appLifecycle = struct {
	mu            sync.Mutex
	clients       map[string]time.Time
	shutdownFunc  func()
	shutdownTimer *time.Timer
}{
	clients: make(map[string]time.Time),
}

func SetLocalAppShutdownFunc(fn func()) {
	appLifecycle.mu.Lock()
	defer appLifecycle.mu.Unlock()
	appLifecycle.shutdownFunc = fn
}

func HandleAppLifecycle(clientID, event string) appLifecycleState {
	id := strings.TrimSpace(clientID)
	normalizedEvent := strings.ToLower(strings.TrimSpace(event))
	if normalizedEvent == "" {
		normalizedEvent = "heartbeat"
	}

	now := time.Now()

	appLifecycle.mu.Lock()
	defer appLifecycle.mu.Unlock()

	pruneAppClientsLocked(now)

	switch normalizedEvent {
	case "open", "heartbeat":
		if id != "" {
			appLifecycle.clients[id] = now
			cancelPendingAppShutdownLocked()
		}
	case "close":
		if id != "" {
			delete(appLifecycle.clients, id)
		}
		if len(appLifecycle.clients) == 0 {
			scheduleAppShutdownLocked()
		}
	default:
		if id != "" {
			appLifecycle.clients[id] = now
			cancelPendingAppShutdownLocked()
		}
	}

	return appLifecycleState{
		ActiveClients: len(appLifecycle.clients),
		Event:         normalizedEvent,
		Shutdown:      appLifecycle.shutdownTimer != nil,
	}
}

func pruneAppClientsLocked(now time.Time) {
	for id, lastSeen := range appLifecycle.clients {
		if now.Sub(lastSeen) > appLifecycleHeartbeatTTL {
			delete(appLifecycle.clients, id)
		}
	}
}

func cancelPendingAppShutdownLocked() {
	if appLifecycle.shutdownTimer == nil {
		return
	}
	appLifecycle.shutdownTimer.Stop()
	appLifecycle.shutdownTimer = nil
}

func scheduleAppShutdownLocked() {
	if appLifecycle.shutdownFunc == nil || appLifecycle.shutdownTimer != nil {
		return
	}
	appLifecycle.shutdownTimer = time.AfterFunc(appShutdownGrace, func() {
		appLifecycle.mu.Lock()
		appLifecycle.shutdownTimer = nil
		pruneAppClientsLocked(time.Now())
		activeCount := len(appLifecycle.clients)
		shutdown := appLifecycle.shutdownFunc
		appLifecycle.mu.Unlock()

		if activeCount > 0 || shutdown == nil {
			return
		}
		log.Printf("no active browser pages, shutting down local backend")
		shutdown()
	})
}
