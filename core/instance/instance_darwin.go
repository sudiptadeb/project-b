package instance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"projectb/timeline"
)

// Owner holds a profile folder: an exclusive flock on the folder's lock file, kept for the
// process's life, and a unix socket that later copies hand their requests to.
type Owner struct {
	lock   *os.File
	socket string
	ln     *net.UnixListener
}

// Claim takes the profile folder dir. It returns the Owner when this copy now owns it. When
// another copy owns it, Claim hands that copy urls and returns nil: this copy then quits.
func Claim(dir string, urls []string) (*Owner, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	socket, err := socketPath(dir)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, err
		}
		return nil, handOver(socket, urls)
	}
	// A socket file left here is a crashed owner's: we hold the lock, so nobody listens on it.
	os.Remove(socket)
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		lock.Close()
		return nil, err
	}
	return &Owner{lock: lock, socket: socket, ln: ln}, nil
}

// socketPath is where the owner of dir listens. A unix socket's path is limited to 104
// bytes on macOS, which a profile folder's own path can pass, so the socket lives in the
// user's temporary folder, named by a hash of the folder's real path.
func socketPath(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(real))
	return filepath.Join(os.TempDir(), "project-b-"+hex.EncodeToString(sum[:8])+".sock"), nil
}

// handOver sends urls to the owner listening at socket.
func handOver(socket string, urls []string) error {
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return err
	}
	defer conn.Close()
	return json.NewEncoder(conn).Encode(request{URLs: urls})
}

// Serve hands each request from a later copy to handle, on a goroutine of its own, until
// Close.
func (o *Owner) Serve(handle func(urls []string)) {
	go func() {
		for {
			conn, err := o.ln.Accept()
			if err != nil {
				return
			}
			var r request
			err = json.NewDecoder(conn).Decode(&r)
			conn.Close()
			if err != nil {
				timeline.Errorf("instance: a request from another copy: %v", err)
				continue
			}
			handle(r.URLs)
		}
	}()
}

// Close stops listening, which removes the socket, and gives the folder up.
func (o *Owner) Close() {
	o.ln.Close()
	o.lock.Close()
}
