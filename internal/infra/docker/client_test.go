package docker_test

import (
	"testing"

	dockerinfra "github.com/jahrulnr/gosite/internal/infra/docker"
	"github.com/stretchr/testify/assert"
)

func TestSanitizeContainerID_Valid(t *testing.T) {
	id, err := dockerinfra.SanitizeContainerID("abc123-def")
	assert.NoError(t, err)
	assert.Equal(t, "abc123-def", id)
}

func TestSanitizeContainerID_Empty(t *testing.T) {
	_, err := dockerinfra.SanitizeContainerID("")
	assert.Error(t, err)
}

func TestSanitizeContainerID_InvalidChars(t *testing.T) {
	_, err := dockerinfra.SanitizeContainerID("abc/../etc")
	assert.Error(t, err)
}

func TestSanitizeContainerID_OnlyHyphen(t *testing.T) {
	id, err := dockerinfra.SanitizeContainerID("a-b-c")
	assert.NoError(t, err)
	assert.Equal(t, "a-b-c", id)
}

func TestSanitizeContainerID_Numeric(t *testing.T) {
	id, err := dockerinfra.SanitizeContainerID("12345")
	assert.NoError(t, err)
	assert.Equal(t, "12345", id)
}

func TestStripDockerLogStream_Payload(t *testing.T) {
	data := []byte{1, 0, 0, 0, 0, 0, 0, 5, 'h', 'e', 'l', 'l', 'o'}
	assert.Equal(t, "hello", dockerinfra.StripDockerLogStream(data))
}

func TestStripDockerLogStream_PlainText(t *testing.T) {
	assert.Equal(t, "plain", dockerinfra.StripDockerLogStream([]byte("plain")))
}

func TestStripDockerLogStream_Empty(t *testing.T) {
	assert.Equal(t, "", dockerinfra.StripDockerLogStream(nil))
}

func TestStripDockerLogStream_MultipleFrames(t *testing.T) {
	frame1 := []byte{1, 0, 0, 0, 0, 0, 0, 2, 'h', 'i'}
	frame2 := []byte{1, 0, 0, 0, 0, 0, 0, 2, '!', '!'}
	data := append(frame1, frame2...)
	assert.Equal(t, "hi!!", dockerinfra.StripDockerLogStream(data))
}

func TestSanitizeContainerID_UnderscoreRejected(t *testing.T) {
	_, err := dockerinfra.SanitizeContainerID("abc_def")
	assert.Error(t, err)
}

func TestSanitizeContainerID_SlashRejected(t *testing.T) {
	_, err := dockerinfra.SanitizeContainerID("abc/def")
	assert.Error(t, err)
}
