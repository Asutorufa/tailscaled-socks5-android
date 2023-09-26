package appctr

import (
	"bufio"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
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
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}

var cmd *exec.Cmd
var sshserver *ssh.Server
var ExecPath string

type Closer interface {
	Close() error
}

func IsRunning() bool { return cmd != nil && cmd.Process != nil }

func Start(sshserver, execPath, sockPath, statePath string, closeCallBack Closer) {
	if IsRunning() {
		return
	}

	ExecPath = execPath

	go func() {
		if err := sshServer(sshserver, filepath.Dir(sockPath)); err != nil {
			log.Println(err)
		}
	}()

	go func() {
		err := tailscaledCmd(execPath, sockPath, statePath)
		if err != nil {
			log.Println(err)
		}

		Stop()

		if closeCallBack != nil {
			closeCallBack.Close()
		}
	}()
}

func Stop() {
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
		cmd = nil
	}

	if sshserver != nil {
		sshserver.Close()
		sshserver = nil
	}
}

func tailscaledCmd(execPath string, socketdir, statedir string) error {
	dataDir := filepath.Dir(socketdir)
	execDir := filepath.Dir(execPath)

	data, err := exec.Command("/system/bin/rm", "-rf",
		filepath.Join(dataDir, "tailscale"),
		filepath.Join(dataDir, "tailscaled")).CombinedOutput()
	log.Println("rm", string(data), err)
	data, err = exec.Command("/system/bin/ln", "-s", filepath.Join(execDir, "libtailscale.so"),
		filepath.Join(dataDir, "tailscale")).CombinedOutput()
	log.Println("ln tailscale", string(data), err)
	data, err = exec.Command("/system/bin/ln", "-s", filepath.Join(execDir, "libtailscaled.so"),
		filepath.Join(dataDir, "tailscaled")).CombinedOutput()
	log.Println("ln tailscaled", string(data), err)

	cmd = exec.Command(
		execPath,
		"--tun=userspace-networking",
		"--socks5-server=:1055",
		"--outbound-http-proxy-listen=:1057",
		fmt.Sprintf("--statedir=%s", statedir),
		fmt.Sprintf("--socket=%s", socketdir),
	)
	os.Setenv("TS_LOGS_DIR", filepath.Join(filepath.Dir(socketdir), "logs"))
	cmd.Env = []string{
		fmt.Sprintf("TS_LOGS_DIR=%s/logs", filepath.Dir(socketdir)),
	}
	cmd.Dir = filepath.Dir(socketdir)
	log.Println(os.Getenv("TS_LOGS_DIR"))

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
			log.Println(s.Text())
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
			log.Println(s.Text())
		}
	}()

	if err := <-errChan; err != nil {
		return err
	}

	return cmd.Run()
}

func sshServer(addr, datadir string) error {
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
			ptyHandler(s, datadir)
		},
	}

	sshserver = &ssh_server

	log.Println("starting ssh server on", addr)
	log.Fatal(ssh_server.ListenAndServe())
	return nil
}

func ptyHandler(s ssh.Session, workDir string) {
	s.Write([]byte{'\n'})
	s.Write(fmt.Appendf(nil, "tailscaled path is %s\n", ExecPath))
	wd, _ := os.Getwd()
	s.Write(fmt.Appendf(nil, "current path is %s\n", wd))
	s.Write([]byte{'\n'})

	log.Println("new session", s.RemoteAddr())

	cmd := exec.Command("/system/bin/sh")
	cmd.Dir = workDir
	ptyReq, winCh, isPty := s.Pty()
	if isPty {
		cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
		f, err := pty.Start(cmd)
		if err != nil {
			log.Println("start pty failed", err)
			return
		}
		go func() {
			for win := range winCh {
				setWinsize(f, win.Width, win.Height)
			}
		}()
		go func() {
			io.Copy(f, s) // stdin
			f.Close()
		}()
		io.Copy(s, f) // stdout
		s.Close()
		cmd.Wait()
		log.Println("session exit", s.RemoteAddr())
	} else {
		io.WriteString(s, "No PTY requested.\n")
		s.Exit(1)
	}
}

// sftpHandler handler for SFTP subsystem
func sftpHandler(sess ssh.Session) {
	log.Println("new sftp session from", sess.RemoteAddr())
	debugStream := io.Discard
	serverOptions := []sftp.ServerOption{
		sftp.WithDebug(debugStream),
	}
	server, err := sftp.NewServer(
		sess,
		serverOptions...,
	)
	if err != nil {
		log.Printf("sftp server init error: %s\n", err)
		return
	}
	if err := server.Serve(); err == io.EOF {
		server.Close()
		fmt.Println("sftp client exited session.")
	} else if err != nil {
		fmt.Println("sftp server completed with error:", err)
	}

	log.Println("sftp session exited", sess.RemoteAddr())
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
