package deployjob

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// GPTMAXXING
func BuildUnit(namespace, service, description, executablePath string) string {
	pair := namespace + "_" + service

	return fmt.Sprintf(`[Unit]
Description=%s
After=network.target

[Service]
Type=simple
EnvironmentFile=/etc/%s/overwrite.env
Environment=DEPLOYD_SERVICE_NAMESPACE=%v
Environment=DEPLOYD_SERVICE=%s
ExecStart=/opt/%s/current/%s
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
`, description, pair, namespace, service, pair, executablePath)
}

// ChatGPTMaxxing
func ExtractTarGzStrip(ctx context.Context, src, dest string) error {
	dest = filepath.Clean(dest)

	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	var rootPrefix string
	buf := make([]byte, 32*1024)

	for {
		// ✅ context cancellation point
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Reject absolute paths
		if filepath.IsAbs(hdr.Name) {
			return fmt.Errorf("absolute path rejected: %s", hdr.Name)
		}

		clean := filepath.Clean(hdr.Name)
		parts := strings.SplitN(clean, string(os.PathSeparator), 2)
		if len(parts) < 2 {
			continue // skip top folder entry
		}

		// enforce single top-level folder
		if rootPrefix == "" {
			rootPrefix = parts[0]
		} else if parts[0] != rootPrefix {
			return fmt.Errorf("multiple top-level folders: %s vs %s", rootPrefix, parts[0])
		}

		relPath := parts[1]
		target := filepath.Join(dest, relPath)
		target = filepath.Clean(target)

		// Prevent traversal
		if !strings.HasPrefix(target, dest+string(os.PathSeparator)) {
			return fmt.Errorf("path traversal detected: %s", hdr.Name)
		}

		switch hdr.Typeflag {

		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			out, err := os.OpenFile(
				target,
				os.O_CREATE|os.O_EXCL|os.O_WRONLY,
				os.FileMode(hdr.Mode)&0755,
			)
			if err != nil {
				return err
			}

			// ✅ context-aware copy
			for {
				select {
				case <-ctx.Done():
					out.Close()
					return ctx.Err()
				default:
				}

				n, rerr := tr.Read(buf)
				if n > 0 {
					if _, werr := out.Write(buf[:n]); werr != nil {
						out.Close()
						return werr
					}
				}
				if rerr != nil {
					if rerr == io.EOF {
						break
					}
					out.Close()
					return rerr
				}
			}

			out.Close()

		case tar.TypeSymlink:
			return fmt.Errorf("symlink rejected: %s", hdr.Name)

		default:
			return fmt.Errorf("unsupported tar entry: %s", hdr.Name)
		}
	}

	if rootPrefix == "" {
		return fmt.Errorf("invalid or empty archive")
	}

	return nil
}

// GPTmaxxing
func Copy(ctx context.Context, dst io.Writer, src io.Reader) error {
	buf := make([]byte, 32*1024)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}
