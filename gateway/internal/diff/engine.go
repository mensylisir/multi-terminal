package diff

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"strings"
)

var (
	// ANSI color codes and escape sequences
	ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
	// Progress bars, spinners, dynamic content
	noiseRegex = regexp.MustCompile(`[\│├└┤┬┴┼─]+\s*$`)
	// Timestamps like [12:34:56]
	timestampRegex = regexp.MustCompile(`\[\d{2}:\d{2}:\d{2}\]`)
)

type DiffResult struct {
	SessionID   uint32
	RawOutput   string
	CleanOutput string
	Lines       []string
	MD5Hash     string
	SHA1Hash    string
	ExitCode    int
	IsDifferent bool
}

type Engine struct {
	ansiRegex       *regexp.Regexp
	noiseRegex      *regexp.Regexp
	timestampRegex  *regexp.Regexp
}

func NewEngine() *Engine {
	return &Engine{
		ansiRegex:      ansiRegex,
		noiseRegex:     noiseRegex,
		timestampRegex: timestampRegex,
	}
}

// StripANSI removes ANSI color and cursor control sequences
func (e *Engine) StripANSI(input string) string {
	return e.ansiRegex.ReplaceAllString(input, "")
}

// FilterNoise removes dynamic content like progress bars and timestamps
func (e *Engine) FilterNoise(input string) string {
	result := e.noiseRegex.ReplaceAllString(input, "")
	result = e.timestampRegex.ReplaceAllString(result, "")
	return result
}

// Process takes raw output and returns a DiffResult
func (e *Engine) Process(sessionID uint32, rawOutput []byte, exitCode int) *DiffResult {
	raw := string(rawOutput)

	// 1. Strip ANSI
	clean := e.StripANSI(raw)

	// 2. Filter noise
	clean = e.FilterNoise(clean)

	// 3. Split into lines
	lines := strings.Split(clean, "\n")

	// 4. Calculate hashes
	md5Hash := e.calcMD5(clean)
	sha1Hash := e.calcSHA1(clean)

	return &DiffResult{
		SessionID:   sessionID,
		RawOutput:   raw,
		CleanOutput: clean,
		Lines:       lines,
		MD5Hash:     md5Hash,
		SHA1Hash:    sha1Hash,
		ExitCode:    exitCode,
	}
}

// Compare compares two DiffResults and returns whether they differ
func (e *Engine) Compare(result1, result2 *DiffResult) bool {
	if result1.ExitCode != result2.ExitCode {
		return true
	}
	if result1.MD5Hash != result2.MD5Hash {
		return true
	}
	return false
}

func (e *Engine) calcMD5(data string) string {
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (e *Engine) calcSHA1(data string) string {
	hash := sha1.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}