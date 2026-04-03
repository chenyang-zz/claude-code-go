package permission

import "testing"

func TestFilesystemRequestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     FilesystemRequest
		wantErr bool
	}{
		{
			name: "valid read request",
			req: FilesystemRequest{
				ToolName:   "file_read",
				Path:       "README.md",
				WorkingDir: "/workspace",
				Access:     AccessRead,
			},
		},
		{
			name: "valid write request",
			req: FilesystemRequest{
				ToolName:   "file_write",
				Path:       "/workspace/out.txt",
				WorkingDir: "/workspace",
				Access:     AccessWrite,
			},
		},
		{
			name: "missing tool name",
			req: FilesystemRequest{
				Path:   "README.md",
				Access: AccessRead,
			},
			wantErr: true,
		},
		{
			name: "missing path",
			req: FilesystemRequest{
				ToolName: "file_read",
				Access:   AccessRead,
			},
			wantErr: true,
		},
		{
			name: "invalid access",
			req: FilesystemRequest{
				ToolName: "file_read",
				Path:     "README.md",
				Access:   Access("execute"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("FilesystemRequest.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
