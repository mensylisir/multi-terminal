package resource

import (
	"fmt"
	"log"
	"os"
	"syscall"
)

const (
	MinFDLimit = 4096
	DefaultMaxSessions = 2000
)

type Guard struct {
	maxFDLimit    uint64
	maxSessions   int
}

func NewGuard(maxSessions int) *Guard {
	return &Guard{
		maxFDLimit:  MinFDLimit,
		maxSessions: maxSessions,
	}
}

// CheckFDLimit verifies file descriptor limit is sufficient
func (g *Guard) CheckFDLimit() error {
	var rlim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlim); err != nil {
		return fmt.Errorf("failed to get FD limit: %w", err)
	}

	if rlim.Cur < MinFDLimit {
		return fmt.Errorf("FD limit too low: current=%d, required=%d. " +
			"Run 'ulimit -n %d' or update /etc/security/limits.conf",
			rlim.Cur, MinFDLimit, MinFDLimit)
	}

	log.Printf("FD limit check passed: current=%d, required=%d", rlim.Cur, MinFDLimit)
	return nil
}

// CheckSystemResources verifies system resources are adequate
func (g *Guard) CheckSystemResources() error {
	// Check FD limit
	if err := g.CheckFDLimit(); err != nil {
		return err
	}

	// Check available memory (at least 256MB)
	var memStat syscall.Statfs_t
	if err := syscall.Statfs("/", &memStat); err != nil {
		return fmt.Errorf("failed to check memory: %w", err)
	}

	availableMem := memStat.Bavail * uint64(memStat.Bsize)
	minMem := int64(256 * 1024 * 1024) // 256MB

	if int64(availableMem) < minMem {
		log.Printf("Warning: Low memory available: %d bytes", availableMem)
	}

	return nil
}

// PreFlight runs all pre-flight checks
func (g *Guard) PreFlight() error {
	log.Println("Running pre-flight checks...")

	if err := g.CheckSystemResources(); err != nil {
		return fmt.Errorf("pre-flight check failed: %w", err)
	}

	log.Println("Pre-flight checks passed")
	return nil
}

// GetCurrentFDUsage returns the current number of open file descriptors
func (g *Guard) GetCurrentFDUsage() (int, error) {
	// Read /proc/self/fd to count open files
	f, err := os.Open("/proc/self/fd")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	for {
		names, err := f.Readdirnames(100)
		if err != nil {
			break
		}
		count += len(names)
		if len(names) < 100 {
			break
		}
	}
	return count, nil
}