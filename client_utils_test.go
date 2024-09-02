package mc_test

import (
	"math/rand"
	"net"
	"os/exec"
	"sync"
	"time"

	"github.com/kinescope/mc"
)

const testServerAddr = "127.1.2.1:11211"

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func newTestClient(addr ...string) (*mc.Client, func(), error) {
	var cancel []func()

	for _, a := range addr {
		c, err := newTestServer(a)
		if err != nil {
			return nil, nil, err
		}
		cancel = append(cancel, c)
	}

	cache, err := mc.New(&mc.Options{
		Addrs: addr,
	})
	if err != nil {
		return nil, nil, err
	}
	return cache, func() {
		for _, fn := range cancel {
			fn()
		}
	}, nil
}

func newTestServer(addr string) (cancel func(), err error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("memcached", "-m", "2", "-p", port, "-l", host)
	if err := cmd.Start(); err != nil {

		return nil, err
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Done()
		cmd.Wait()
	}()
	wg.Wait()
	for n := range 10 {
		c, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			c.Close()
			break
		}
		time.Sleep(time.Duration(25*n) * time.Millisecond)
	}
	return func() {
		cmd.Process.Kill()
	}, nil
}
