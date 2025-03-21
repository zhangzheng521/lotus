package build

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	rice "github.com/GeertJohan/go.rice"
	logging "github.com/ipfs/go-log"
	"github.com/minio/blake2b-simd"
	"go.uber.org/multierr"
	"golang.org/x/xerrors"
	pb "gopkg.in/cheggaaa/pb.v1"
)

var log = logging.Logger("build")

//const gateway = "http://198.211.99.118/ipfs/"
const gateway = "https://ipfs.greyh.at/ipfs/"
//const gateway = "https://ipfs.io/ipfs/"
const paramdir = "/var/tmp/filecoin-proof-parameters"
const dirEnv = "FIL_PROOFS_PARAMETER_CACHE"

type paramFile struct {
	Cid        string `json:"cid"`
	Digest     string `json:"digest"`
	SectorSize uint64 `json:"sector_size"`
}

type fetch struct {
	wg      sync.WaitGroup
	fetchLk sync.Mutex

	errs []error
}

func getParamDir() string {
	if os.Getenv(dirEnv) == "" {
		return paramdir
	}
	return os.Getenv(dirEnv)
}

func GetParams(storageSize uint64) error {
	if err := os.Mkdir(getParamDir(), 0755); err != nil && !os.IsExist(err) {
		return err
	}

	var params map[string]paramFile

	paramBytes := rice.MustFindBox("proof-params").MustBytes("parameters.json")
	if err := json.Unmarshal(paramBytes, &params); err != nil {
		return err
	}

	ft := &fetch{}

	for name, info := range params {
		if storageSize != info.SectorSize && strings.HasSuffix(name, ".params") {
			continue
		}

		ft.maybeFetchAsync(name, info)
	}

	return ft.wait()
}

func (ft *fetch) maybeFetchAsync(name string, info paramFile) {
	ft.wg.Add(1)

	go func() {
		defer ft.wg.Done()

		path := filepath.Join(getParamDir(), name)

		err := ft.checkFile(path, info)
		if !os.IsNotExist(err) && err != nil {
			log.Warn(err)
		}
		if err == nil {
			return
		}

		ft.fetchLk.Lock()
		defer ft.fetchLk.Unlock()

		if err := doFetch(path, info); err != nil {
			ft.errs = append(ft.errs, xerrors.Errorf("fetching file %s failed: %w", path, err))
			return
		}
		err = ft.checkFile(path, info)
		if err != nil {
			ft.errs = append(ft.errs, xerrors.Errorf("checking file %s failed: %w", path, err))
			err := os.Remove(path)
			if err != nil {
				ft.errs = append(ft.errs, xerrors.Errorf("remove file %s failed: %w", path, err))
			}
		}
	}()
}

func (ft *fetch) checkFile(path string, info paramFile) error {
	if os.Getenv("TRUST_PARAMS") == "1" {
		log.Warn("Assuming parameter files are ok. DO NOT USE IN PRODUCTION")
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := blake2b.New512()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	sum := h.Sum(nil)
	strSum := hex.EncodeToString(sum[:16])
	if strSum == info.Digest {
		log.Infof("Parameter file %s is ok", path)
		return nil
	}

	return xerrors.Errorf("checksum mismatch in param file %s, %s != %s", path, strSum, info.Digest)
}

func (ft *fetch) wait() error {
	ft.wg.Wait()
	return multierr.Combine(ft.errs...)
}

func doFetch(out string, info paramFile) error {
	gw := os.Getenv("IPFS_GATEWAY")
	if gw == "" {
		gw = gateway
	}
	log.Infof("Fetching %s from %s", out, gw)

	outf, err := os.OpenFile(out, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	defer outf.Close()

	fStat, err := outf.Stat()
	if err != nil {
		return err
	}
	header := http.Header{}
	header.Set("Range", "bytes="+strconv.FormatInt(fStat.Size(), 10)+"-")
	url, err := url.Parse(gw + info.Cid)
	if err != nil {
		return err
	}
	log.Infof("GET %s", url)

	req := http.Request{
		Method: "GET",
		URL:    url,
		Header: header,
		Close:  true,
	}

	resp, err := http.DefaultClient.Do(&req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bar := pb.New64(resp.ContentLength)
	bar.Units = pb.U_BYTES
	bar.ShowSpeed = true
	bar.Start()

	_, err = io.Copy(outf, bar.NewProxyReader(resp.Body))

	bar.Finish()

	return err
}
