package docker

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDisplayProgress(t *testing.T) {
	tests := []struct {
		name           string
		progress       pullProgress
		expectedOutput string
	}{
		{
			name: "pulling from repository",
			progress: pullProgress{
				Status: "Pulling from rwx/my-image",
			},
			expectedOutput: "Pulling from rwx/my-image\n",
		},
		{
			name: "pulling from library",
			progress: pullProgress{
				Status: "Pulling from library/ubuntu",
			},
			expectedOutput: "Pulling from library/ubuntu\n",
		},
		{
			name: "downloading with progress",
			progress: pullProgress{
				Status:   "Downloading",
				ID:       "abc123",
				Progress: "[==>  ] 1.5MB/3MB",
			},
			expectedOutput: "abc123: Downloading [==>  ] 1.5MB/3MB\n",
		},
		{
			name: "extracting with progress",
			progress: pullProgress{
				Status:   "Extracting",
				ID:       "abc123",
				Progress: "[=====>] 2MB/2MB",
			},
			expectedOutput: "abc123: Extracting [=====>] 2MB/2MB\n",
		},
		{
			name: "download complete",
			progress: pullProgress{
				Status: "Download complete",
				ID:     "abc123",
			},
			expectedOutput: "abc123: Download complete\n",
		},
		{
			name: "pull complete",
			progress: pullProgress{
				Status: "Pull complete",
				ID:     "abc123",
			},
			expectedOutput: "abc123: Pull complete\n",
		},
		{
			name: "already exists",
			progress: pullProgress{
				Status: "Already exists",
				ID:     "abc123",
			},
			expectedOutput: "abc123: Already exists\n",
		},
		{
			name: "digest",
			progress: pullProgress{
				Status: "Digest:",
				ID:     "sha256:abc123",
			},
			expectedOutput: "Digest: sha256:abc123\n",
		},
		{
			name: "status",
			progress: pullProgress{
				Status: "Status:",
				ID:     "Downloaded newer image",
			},
			expectedOutput: "Status: Downloaded newer image\n",
		},
		{
			name: "downloading without progress - no output",
			progress: pullProgress{
				Status: "Downloading",
				ID:     "abc123",
			},
			expectedOutput: "",
		},
		{
			name: "unknown status - no output",
			progress: pullProgress{
				Status: "Unknown status",
			},
			expectedOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := displayProgress(&buf, &tt.progress)
			require.NoError(t, err)
			require.Equal(t, tt.expectedOutput, buf.String())
		})
	}
}
