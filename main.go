package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const BUF_SIZE = (8 * 1024) + 1 // misalign buffer size
const FILE = "./random_52428800_byte_file"
const FILE_DEFAULT = 11342319 // this cauess 3 minutes of stall
const FILE_MAX = 52428800
const PAUSE_TIME = 7944 * time.Microsecond
const THINK_TIME = PAUSE_TIME * 1385

func min(x, y uint64) uint64 {
	if x < y {
		return x
	}
	return y
}

func minInt64(x, y int64) int64 {
	if x < y {
		return x
	}
	return y
}

func maxInt64(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}

// hacked up https://golang.org/src/io/io.go?s=13817:13895#L422
var errInvalidWrite = errors.New("invalid write result")

func copyBy(dst io.Writer, src io.Reader, size uint64, withWait bool) (written int64, err error) {
	buf := make([]byte, size)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errInvalidWrite
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
		if withWait {
			time.Sleep(PAUSE_TIME)
		}
	}
	return written, err
}

func serve(w http.ResponseWriter, r *http.Request) {
	log.Printf("serve: remote = %s, path = %s", r.RemoteAddr, r.URL.Path)
	switch r.URL.Path {
	case "/think", "/drip":
		withCopyWait := r.URL.Path == "/drip"
		if r.URL.Path == "/think" {
			thinkTime, err := strconv.ParseInt(r.URL.Query().Get("thinktime"), 10, 64)
			if thinkTime < 0 || err != nil {
				thinkTime = int64(THINK_TIME)
			}
			thinkTime = minInt64(int64(THINK_TIME), int64(time.Duration(thinkTime)*time.Microsecond))
			log.Printf("thinking for %dns", thinkTime)
			time.Sleep(time.Duration(thinkTime))
		}

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Expires", "Thu, 19 Nov 1981 08:52:00 GMT")
		w.Header().Set("Cache-Control", "private, max-age=0, no-store, no-cache, must-revalidate")

		f, err := os.Open(FILE)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer f.Close()

		w.WriteHeader(http.StatusOK)

		limit, _ := strconv.ParseUint(r.URL.Query().Get("limit"), 10, 64)
		if limit == 0 {
			limit = FILE_DEFAULT
		} else {
			limit = min(limit, FILE_MAX)
		}

		log.Printf("serve: withCopyWait: %t, limit = %d", withCopyWait, limit)
		lr := &io.LimitedReader{R: f, N: int64(limit)}
		copyBy(w, lr, BUF_SIZE, withCopyWait)
	default:
		fmt.Fprintf(w, "moo")
	}
}

func main() {
	log.Println("listen: :80")
	http.HandleFunc("/", serve)
	err := http.ListenAndServe(":80", nil)
	if err != nil {
		log.Fatal(err)
	}
}
