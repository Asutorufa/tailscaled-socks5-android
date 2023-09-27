package appctr

import (
	"bufio"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
	_ "golang.org/x/mobile/bind"
)

func setWinsize(f *os.File, w, h int) {
	_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}

var cmd *exec.Cmd
var sshserver *ssh.Server
var PC pathControl

type Closer interface {
	Close() error
}

func IsRunning() bool { return cmd != nil && cmd.Process != nil }

func Start(sshserver, execPath, sockPath, statePath string, closeCallBack Closer) {
	if IsRunning() {
		return
	}

	PC = newPathControl(execPath, sockPath, statePath)

	go func() {
		if err := sshServer(sshserver, PC); err != nil {
			slog.Error("ssh server", "err", err)
		}
	}()

	go func() {
		err := tailscaledCmd(PC)
		if err != nil {
			slog.Error("tailscaled cmd", "err", err)
		}

		Stop()

		if closeCallBack != nil {
			closeCallBack.Close()
		}
	}()
}

func Stop() {
	if sshserver != nil {
		slog.Info("stop ssh server")
		sshserver.Close()
		sshserver = nil
	}

	if cmd != nil && cmd.Process != nil {
		slog.Info("stop tailscaled cmd")
		_ = cmd.Process.Kill()
		cmd = nil
	}

}

func rm(path ...string) {
	if len(path) == 0 {
		return
	}

	args := []string{"-rf"}
	args = append(args, path...)
	data, err := exec.Command("/system/bin/rm", args...).CombinedOutput()
	slog.Info("rm", "cmd", args, "output", string(data), "err", err)
}

func ln(src, dst string) {
	cmd := exec.Command("/system/bin/ln", "-s", src, dst)
	data, err := cmd.CombinedOutput()
	slog.Info("ln", "cmd", cmd.String(), "output", string(data), "err", err)
}

type pathControl struct {
	execPath   string
	statePath  string
	socketPath string
	execDir    string
	dataDir    string
}

func newPathControl(execPath, socketPath, statePath string) pathControl {
	return pathControl{
		execPath:   execPath,
		statePath:  statePath,
		socketPath: socketPath,
		execDir:    filepath.Dir(execPath),
		dataDir:    filepath.Dir(socketPath),
	}
}

func (p pathControl) TailscaledSo() string { return p.execPath }
func (p pathControl) Tailscaled() string   { return filepath.Join(p.dataDir, "tailscaled") }
func (p pathControl) TailscaleSo() string  { return filepath.Join(p.execDir, "libtailscale.so") }
func (p pathControl) Tailscale() string    { return filepath.Join(p.dataDir, "tailscale") }
func (p pathControl) DataDir(s ...string) string {
	if len(s) == 0 {
		return p.dataDir
	}
	return filepath.Join(append([]string{p.dataDir}, s...)...)
}
func (p pathControl) Socket() string { return p.socketPath }
func (p *pathControl) State() string { return p.statePath }

func tailscaledCmd(p pathControl) error {

	rm(p.Tailscale(), p.Tailscaled())
	ln(p.TailscaleSo(), p.Tailscale())
	ln(p.TailscaledSo(), p.Tailscaled())

	cmd = exec.Command(
		p.TailscaledSo(),
		"--tun=userspace-networking",
		"--socks5-server=:1055",
		"--outbound-http-proxy-listen=:1057",
		fmt.Sprintf("--statedir=%s", p.State()),
		fmt.Sprintf("--socket=%s", p.Socket()),
	)
	cmd.Dir = p.DataDir()
	cmd.Env = []string{
		fmt.Sprintf("TS_LOGS_DIR=%s/logs", p.DataDir()),
	}

	errChan := make(chan error)
	defer close(errChan)

	go func() {
		stdOut, err := cmd.StdoutPipe()
		if err != nil {
			errChan <- err
			return
		}
		defer stdOut.Close()

		errChan <- nil

		s := bufio.NewScanner(stdOut)

		for s.Scan() {
			slog.Info(s.Text())
		}
	}()

	if err := <-errChan; err != nil {
		return err
	}

	go func() {
		stdOut, err := cmd.StderrPipe()
		if err != nil {
			errChan <- err
			return
		}
		defer stdOut.Close()

		errChan <- nil

		s := bufio.NewScanner(stdOut)

		for s.Scan() {
			slog.Info(s.Text())
		}
	}()

	if err := <-errChan; err != nil {
		return err
	}

	return cmd.Run()
}

func sshServer(addr string, pc pathControl) error {
	p, _ := pem.Decode([]byte(PrivateKey))
	key, _ := x509.ParsePKCS1PrivateKey(p.Bytes)

	signer, err := gossh.NewSignerFromKey(key)
	if err != nil {
		return err
	}

	ssh_server := ssh.Server{
		Addr:        addr,
		HostSigners: []ssh.Signer{signer},
		SubsystemHandlers: map[string]ssh.SubsystemHandler{
			"sftp": sftpHandler,
		},
		Handler: func(s ssh.Session) {
			ptyHandler(s, pc)
		},
	}

	sshserver = &ssh_server

	slog.Info("starting ssh server", "host", addr)
	slog.Info("ssh server", "err", ssh_server.ListenAndServe())
	return nil
}

var ptyWelcome = `
Welcome to Tailscaled SSH
	Tailscaled: %s
	Work Dir: %s
	RemoteAddr: %s
`

func ptyHandler(s ssh.Session, pc pathControl) {
	_, _ = fmt.Fprintf(s, ptyWelcome, pc.TailscaledSo(), pc.DataDir(), s.RemoteAddr())

	slog.Info("new pty session", "remote addr", s.RemoteAddr())

	cmd := exec.Command("/system/bin/sh")
	cmd.Dir = pc.DataDir()
	ptyReq, winCh, isPty := s.Pty()
	if isPty {
		cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
		f, err := pty.Start(cmd)
		if err != nil {
			slog.Error("start pty", "err", err)
			return
		}
		go func() {
			for win := range winCh {
				setWinsize(f, win.Width, win.Height)
			}
		}()
		go func() {
			_, _ = io.Copy(f, s) // stdin
			f.Close()
		}()
		_, _ = io.Copy(s, f) // stdout
		s.Close()
		err = cmd.Wait()
		slog.Info("session exit", "remote addr", s.RemoteAddr(), "wait error", err)
	} else {
		_, _ = io.WriteString(s, "No PTY requested.\n")
		_ = s.Exit(1)
	}
}

// sftpHandler handler for SFTP subsystem
func sftpHandler(sess ssh.Session) {
	slog.Info("new sftp session", "remote addr", sess.RemoteAddr())
	debugStream := io.Discard
	serverOptions := []sftp.ServerOption{
		sftp.WithDebug(debugStream),
	}
	server, err := sftp.NewServer(
		sess,
		serverOptions...,
	)
	if err != nil {
		slog.Error("sftp server init", "err", err)
		return
	}
	if err := server.Serve(); err == io.EOF {
		server.Close()
		slog.Info("sftp client exited session.")
	} else if err != nil {
		slog.Error("sftp server completed", "err", err)
	}

	slog.Info("sftp session exited", "remote addr", sess.RemoteAddr())
}

var PrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEA0ludCFgG93sH//e/5CxAFG/PTjclfTDKaXpl2EBZQmoZSfJE
/4UCy/tUynYEovEbBT3PDyw7LmVnnyktetV4ra3OtBM0iNdZnZeIvW0kQaC7alGe
3sHzNfTxT60w3drrjLzOX+Wd9E/WZ4ScXzhgJ8kzqtRBbGgSukTSxgPgz89XiIDI
P+j721TrQYgOOnje4+SGEWL3TTAENDayk6RWom/58LstYz42TdaBFWHz+W2nG+Dw
EkzywnKGuMpccdNOvEnaoxVUZ+6OC7qOGfjSKBJIDmj39+XaVeduLMbjAy8cGAl4
n3HFTBMPupRqC6sbSrgpZ4MiNfrElF4cDXrBWQIDAQABAoIBAF6klVxhrpC+K/VA
VHemaRZIz+6S5S0UPJ2EUjofiYlWDxa0B9Mm1wFLjPSicKeW7t9G1dgvwFi5iwuT
DUFMtkT+BBgE5AgFS+6ZdQ41ArD8ThYhrubuQCywjbmZZHkMvBnQANIojw6StRZS
FcDJrol3/uUHJoBNus9Pk70/lXApOfgy+Yg3RTIPy+AMHr4exSGEGATFMFrOyiit
+xYBSnHFQzt63UsLUL5zWDFcljH5SmQJAkoCtrN3oiZRBb4v3TWYzSvDIR8BTuoD
Fj/EI9kWFyzx0MBpfOcU+ggNw1KSX+fsyRrMnirFg7HB7F7wL9MiJbihUVnbxDs8
Fy4vr2ECgYEA+tCmmr258UQcdxll0GgNz/WcY03HJlcZkfnnUpJKyb+K3Lpc5ekz
BrXKYNZ0A4gm8/8P55ykIPbWH2mi6/cDIkvsoGYOCac5P2Nf+W54wWtFzjuKx+x6
aoIKOCoQr69XM+KrbwZoku8g6VatRsJ1oMupZM3ucaTOBCm0hKVPAy0CgYEA1rTb
f/F7K8eJUbmX7dK3tdUuDxDMnf9Zp47JGZOJFuwZvHhZ7nOPw2HntSwVIk651pxc
kZfOWswPPHRxLztnEtBnwskM8e8RHMJHTeLalpHkdBoWT7KQq+uIU0KhiuRiqPcu
I16tZr0ciCgn9QmmtCJH+bpATMy5ZpDTfNHpQl0CgYEA4pzUeulC8F8WzPDwkb0C
BcwnMX3bmqOFoePGAk/VLLVYNJhZSQ1LIhvsL1Rz26EPeNMSPrTDglkjG5ypLEOw
3DL3J/Eta8FgMwqJc2dByZgvqOcZPAtIi6TUsOwoyWNGCcYaGKUUpPVTqh+7TTxz
ZQW+FisN7jX2QcKgrFxjqD0CgYEAgALAxB2T1FxZcRJ4lOEHizAZD/5yINl3+MDX
AZrHJ5WJGqee5t6bnmAnKAuqZhQOFPiQ8HVUISp9Awxh10lRgRQkaSw5vZ1N1Jm4
raVNsmw1i0tqdgX+36HEW+/kJM1aTWdiaNAwDos+EafvetdQPyIZS7lSUPfWqmI6
1bbJnjkCgYEA6M9HYlnaVPAHfguDugSeLJOia46Ui7aJh43znLlU/PoUdRRoBUmi
hUwJg5EHLSdbFj6vtwhqdnUwcH8v3HYK4vbUVamvCYF6kKCRmL2lyz9SH6yxHcPJ
zeMifjk2UYMZK8A0Ik7GxsHfseOx9QeWRbX8VR9QPuuwpGMVdQkeBgA=
-----END RSA PRIVATE KEY-----`
