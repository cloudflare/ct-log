// Package config implements the unmarshalling of config files, into CT,
// Trillian, and database configuration.
package config

import (
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/certificate-transparency-go/trillian/ctfe/configpb"
	"github.com/google/trillian"
	"github.com/google/trillian/crypto/keyspb"
	spb "github.com/google/trillian/crypto/sigpb"
	"github.com/google/trillian/storage"
	"gopkg.in/yaml.v2"
)

// file specifies the file format of our config. See config.dev.yml for
// examles/annotations.
type file struct {
	MetricsAddr string `yaml:"metrics_addr"`
	ServerAddr  string `yaml:"server_addr"`
	CertFile    string `yaml:"cert_file"`
	KeyFile     string `yaml:"key_file"`

	BoltPath string `yaml:"bolt_path"`

	B2AcctId string `yaml:"b2_acct_id"`
	B2AppKey string `yaml:"b2_app_key"`
	B2Bucket string `yaml:"b2_bucket"`
	B2Url    string `yaml:"b2_url"`

	MaxUnsequencedLeaves int64  `yaml:"max_unsequenced_leaves"`
	MaxClients           int    `yaml:"max_clients"`
	RequestTimeout       string `yaml:"request_timeout"`

	Signer struct {
		BatchSize   int           `yaml:"batch_size"`
		RunInterval time.Duration `yaml:"run_interval"`
		GuardWindow time.Duration `yaml:"guard_window"`
	} `yaml:"signer"`
	Logs []logMeta `yaml:"logs"`
}

type logMeta struct {
	LogId      int64  `yaml:"log_id"`
	CreateTime string `yaml:"create_time"`
	UpdateTime string `yaml:"update_time"`

	TreeState       string `yaml:"tree_state"`
	SigAlg          string `yaml:"sig_alg"`
	MaxRootDuration string `yaml:"max_root_duration"`

	NotAfterStart string `yaml:"not_after_start"`
	NotAfterStop  string `yaml:"not_after_stop"`

	Prefix    string `yaml:"prefix"`
	RootsFile string `yaml:"roots_file"`

	PubKey  string `yaml:"pub_key"`
	PrivKey string `yaml:"priv_key"`
}

type Config struct {
	MetricsAddr string
	ServerAddr  string
	CertFile    string
	KeyFile     string

	BoltPath string

	B2AcctId string
	B2AppKey string
	B2Bucket string
	B2Url    string

	MaxUnsequencedLeaves int64
	MaxClients           int
	RequestTimeout       time.Duration

	Signer       SignerConfig
	LogConfigs   []*configpb.LogConfig
	AdminStorage storage.AdminStorage
}

type SignerConfig struct {
	BatchSize   int
	RunInterval time.Duration
	GuardWindow time.Duration
}

func FromFile(path string) (*Config, error) {
	// Read config from file and decode.
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	parsed := &file{}
	if err = yaml.Unmarshal(raw, parsed); err != nil {
		return nil, err
	}

	if len(parsed.MetricsAddr) == 0 {
		return nil, fmt.Errorf("no address to serve metrics on was found in config file")
	} else if len(parsed.ServerAddr) == 0 {
		return nil, fmt.Errorf("no address for the server to listen on was found in config file")
	}

	if len(parsed.BoltPath) == 0 {
		return nil, fmt.Errorf("boltdb path not found in config file")
	} else if len(parsed.B2AcctId) == 0 {
		return nil, fmt.Errorf("no backblaze account id found in config file")
	} else if len(parsed.B2AppKey) == 0 {
		return nil, fmt.Errorf("no backblaze application key found in config file")
	} else if len(parsed.B2Bucket) == 0 {
		return nil, fmt.Errorf("no backblaze bucket found in config file")
	} else if len(parsed.B2Url) == 0 {
		return nil, fmt.Errorf("no backblaze download url found in config file")
	}

	if parsed.MaxUnsequencedLeaves < 1 {
		return nil, fmt.Errorf("max_unsequenced_leaves must be positive")
	} else if parsed.MaxClients < 1 {
		return nil, fmt.Errorf("max_clients cannot be less than one")
	}
	requestTimeout, err := time.ParseDuration(parsed.RequestTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to parse request timeout: %v", err)
	}

	if len(parsed.Logs) == 0 {
		return nil, fmt.Errorf("no logs found in config file")
	}

	// Verify that all log ids are distinct and well-formed.
	for i, meta := range parsed.Logs {
		if meta.LogId <= 0 {
			return nil, fmt.Errorf("log #%v in config file: log cannot have id %v", i+1, meta.LogId)
		}

		for j, cand := range parsed.Logs[i+1:] {
			if meta.LogId == cand.LogId {
				return nil, fmt.Errorf("logs #%v and #%v in config file have the same log id", i+1, j+1)
			}
		}
	}

	// Extract the CT-related configuration from each block of config.
	logConfigs := make([]*configpb.LogConfig, 0, len(parsed.Logs))
	for i, meta := range parsed.Logs {
		cfg, err := logConfig(meta)
		if err != nil {
			return nil, fmt.Errorf("log #%v in config file: %v", i+1, err)
		}
		logConfigs = append(logConfigs, cfg)
	}

	// Extract the Trillian-related configuration from each block.
	trees := make([]*trillian.Tree, 0, len(parsed.Logs))
	for i, meta := range parsed.Logs {
		tree, err := readTree(meta)
		if err != nil {
			return nil, fmt.Errorf("log #%v in config file: %v", i+1, err)
		}
		trees = append(trees, tree)
	}

	return &Config{
		MetricsAddr: parsed.MetricsAddr,
		ServerAddr:  parsed.ServerAddr,
		CertFile:    parsed.CertFile,
		KeyFile:     parsed.KeyFile,

		BoltPath: parsed.BoltPath,

		B2AcctId: os.ExpandEnv(parsed.B2AcctId),
		B2AppKey: os.ExpandEnv(parsed.B2AppKey),
		B2Bucket: os.ExpandEnv(parsed.B2Bucket),
		B2Url:    os.ExpandEnv(parsed.B2Url),

		MaxUnsequencedLeaves: parsed.MaxUnsequencedLeaves,
		MaxClients:           parsed.MaxClients,
		RequestTimeout:       requestTimeout,

		Signer: SignerConfig{
			BatchSize:   parsed.Signer.BatchSize,
			RunInterval: parsed.Signer.RunInterval,
			GuardWindow: parsed.Signer.GuardWindow,
		},
		LogConfigs:   logConfigs,
		AdminStorage: &adminStorage{trees},
	}, nil
}

func logConfig(meta logMeta) (*configpb.LogConfig, error) {
	pubKey, privKey, err := parseKeypair(meta)
	if err != nil {
		return nil, err
	}
	var notAfterStart, notAfterStop *timestamp.Timestamp
	if meta.NotAfterStart != "" || meta.NotAfterStop != "" {
		start, err := parseTime(meta.NotAfterStart)
		if err != nil {
			return nil, err
		}
		stop, err := parseTime(meta.NotAfterStop)
		if err != nil {
			return nil, err
		}
		notAfterStart = &timestamp.Timestamp{Seconds: start.Unix()}
		notAfterStop = &timestamp.Timestamp{Seconds: stop.Unix()}
	}

	return &configpb.LogConfig{
		LogId:        meta.LogId,
		Prefix:       meta.Prefix,
		RootsPemFile: []string{meta.RootsFile},

		NotAfterStart: notAfterStart,
		NotAfterLimit: notAfterStop,

		PublicKey:  pubKey,
		PrivateKey: privKey,
	}, nil
}

func readTree(meta logMeta) (*trillian.Tree, error) {
	tree := &trillian.Tree{
		TreeId: meta.LogId,

		TreeType:      trillian.TreeType_LOG,
		HashStrategy:  trillian.HashStrategy_RFC6962_SHA256,
		HashAlgorithm: spb.DigitallySigned_SHA256,
	}

	if ts, ok := trillian.TreeState_value[meta.TreeState]; ok {
		tree.TreeState = trillian.TreeState(ts)
	} else {
		return nil, fmt.Errorf("unknown tree state: %v", meta.TreeState)
	}

	if sa, ok := spb.DigitallySigned_SignatureAlgorithm_value[meta.SigAlg]; ok {
		tree.SignatureAlgorithm = spb.DigitallySigned_SignatureAlgorithm(sa)
	} else {
		return nil, fmt.Errorf("unknown signature algorithm: %v", meta.SigAlg)
	}

	pubKey, privKey, err := parseKeypair(meta)
	if err != nil {
		return nil, err
	}
	tree.PublicKey, tree.PrivateKey = pubKey, privKey

	maxRootDuration, err := time.ParseDuration(meta.MaxRootDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to parse max root duration: %v", err)
	}
	tree.MaxRootDuration = ptypes.DurationProto(maxRootDuration)

	createTime, err := parseTime(meta.CreateTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse create time: %v", err)
	}
	tree.CreateTime, err = ptypes.TimestampProto(createTime)
	if err != nil {
		return nil, err
	}

	updateTime, err := parseTime(meta.UpdateTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse update time: %v", err)
	}
	tree.UpdateTime, err = ptypes.TimestampProto(updateTime)
	if err != nil {
		return nil, err
	}

	return tree, nil
}

func parseKeypair(meta logMeta) (*keyspb.PublicKey, *any.Any, error) {
	pubStr, privStr := os.ExpandEnv(meta.PubKey), os.ExpandEnv(meta.PrivKey)

	// Parse the public key.
	pubBlock, rest := pem.Decode([]byte(pubStr))
	if pubBlock == nil {
		return nil, nil, fmt.Errorf("failed to pem-decode public key")
	} else if len(rest) > 0 {
		return nil, nil, fmt.Errorf("unnecessary data appended to public key")
	}
	pubKey := &keyspb.PublicKey{Der: pubBlock.Bytes}

	// Parse the private key.
	privBlock, rest := pem.Decode([]byte(privStr))
	if privBlock == nil {
		return nil, nil, fmt.Errorf("failed to pem-decode private key")
	} else if len(rest) > 0 {
		return nil, nil, fmt.Errorf("unnecessary data appended to private key")
	}
	privKey, err := ptypes.MarshalAny(&keyspb.PrivateKey{Der: privBlock.Bytes})
	if err != nil {
		return nil, nil, err
	}

	return pubKey, privKey, nil
}

func parseTime(in string) (time.Time, error) {
	return time.Parse("2006-01-02 15:04:05 MST", in)
}
