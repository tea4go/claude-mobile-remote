package push

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"
)

const maxDeviceRoutes = 4096
const maxDevicesPerTopic = 10

var registrationIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{8,64}$`)

// DeviceRegistry stores only notification topics and JPush registration IDs.
// Its file must live in a private, service-writable directory.
type DeviceRegistry struct {
	mu     sync.Mutex
	path   string
	routes map[string]map[string]time.Time
}

func LoadDeviceRegistry(path string) (*DeviceRegistry, error) {
	if path == "" {
		return nil, errors.New("notification device file is required")
	}
	r := &DeviceRegistry{path: path, routes: make(map[string]map[string]time.Time)}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 1024*1024 {
		return nil, errors.New("notification device file must be private and at most 1 MiB")
	}
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &r.routes) != nil {
		return nil, errors.New("cannot load notification device file")
	}
	if r.routes == nil {
		r.routes = make(map[string]map[string]time.Time)
	}
	if routeCount(r.routes) > maxDeviceRoutes {
		return nil, errors.New("notification device file capacity exceeded")
	}
	now := time.Now()
	for topic, devices := range r.routes {
		if !topicPattern.MatchString(topic) || len(devices) > maxDevicesPerTopic {
			return nil, errors.New("invalid notification device file")
		}
		for rid, expiry := range devices {
			if !registrationIDPattern.MatchString(rid) || expiry.After(now.Add(30*24*time.Hour+time.Minute)) {
				return nil, errors.New("invalid notification device file")
			}
		}
	}
	return r, nil
}

func validDeviceRoute(topic, rid string, expiry time.Time, now time.Time) bool {
	return topicPattern.MatchString(topic) && registrationIDPattern.MatchString(rid) &&
		expiry.After(now) && !expiry.After(now.Add(30*24*time.Hour+time.Minute))
}

func (r *DeviceRegistry) Register(topic, rid string, expiry time.Time) error {
	now := time.Now()
	if !validDeviceRoute(topic, rid, expiry, now) {
		return errors.New("invalid notification device registration")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	next := r.liveCopy(now)
	if next[topic] == nil {
		next[topic] = make(map[string]time.Time)
	}
	if _, existing := next[topic][rid]; !existing && len(next[topic]) >= maxDevicesPerTopic {
		return errors.New("notification topic device limit reached")
	}
	next[topic][rid] = expiry.UTC()
	if routeCount(next) > maxDeviceRoutes {
		return errors.New("notification device capacity reached")
	}
	if err := r.save(next); err != nil {
		return err
	}
	r.routes = next
	return nil
}

func (r *DeviceRegistry) Remove(topic, rid string) error {
	if !topicPattern.MatchString(topic) || !registrationIDPattern.MatchString(rid) {
		return errors.New("invalid notification device registration")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	next := r.liveCopy(time.Now())
	delete(next[topic], rid)
	if len(next[topic]) == 0 {
		delete(next, topic)
	}
	if err := r.save(next); err != nil {
		return err
	}
	r.routes = next
	return nil
}

func (r *DeviceRegistry) Targets(topic string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var ids []string
	for rid, expiry := range r.routes[topic] {
		if expiry.After(time.Now()) {
			ids = append(ids, rid)
		}
	}
	sort.Strings(ids)
	return ids
}

func (r *DeviceRegistry) liveCopy(now time.Time) map[string]map[string]time.Time {
	next := make(map[string]map[string]time.Time, len(r.routes))
	for topic, devices := range r.routes {
		for rid, expiry := range devices {
			if expiry.After(now) {
				if next[topic] == nil {
					next[topic] = make(map[string]time.Time)
				}
				next[topic][rid] = expiry
			}
		}
	}
	return next
}

func routeCount(routes map[string]map[string]time.Time) int {
	count := 0
	for _, devices := range routes {
		count += len(devices)
	}
	return count
}

func (r *DeviceRegistry) save(routes map[string]map[string]time.Time) error {
	data, err := json.Marshal(routes)
	if err != nil || len(data) > 1024*1024 {
		return errors.New("notification device file capacity reached")
	}
	f, err := os.CreateTemp(filepath.Dir(r.path), ".devices-*")
	if err != nil {
		return errors.New("cannot save notification devices")
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(f.Name(), r.path)
	}
	if err != nil {
		return errors.New("cannot save notification devices")
	}
	return nil
}
