module github.com/0xsequence/quotacontrol

go 1.24.3

// replace github.com/0xsequence/authcontrol => ../authcontrol

require (
	github.com/0xsequence/authcontrol v0.3.8
	github.com/0xsequence/go-sequence v0.44.2
	github.com/alicebob/miniredis/v2 v2.33.0
	github.com/go-chi/chi/v5 v5.1.0
	github.com/go-chi/httprate v0.14.1
	github.com/go-chi/httprate-redis v0.5.4
	github.com/goware/base64 v0.1.0
	github.com/goware/logger v0.3.0
	github.com/goware/validation v0.1.3
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/jxskiss/base62 v1.1.0
	github.com/redis/go-redis/v9 v9.7.0
	github.com/stretchr/testify v1.10.0
)

require (
	dario.cat/mergo v1.0.1 // indirect
	github.com/0xsequence/ethkit v1.30.2 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.3.1 // indirect
	github.com/Masterminds/sprig/v3 v3.3.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/go-crypto v1.1.5 // indirect
	github.com/alicebob/gopher-json v0.0.0-20230218143504-906a9b012302 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudflare/circl v1.6.0 // indirect
	github.com/cyphar/filepath-securejoin v0.4.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-chi/jwtauth/v5 v5.3.2 // indirect
	github.com/go-chi/traceid v0.2.0 // indirect
	github.com/go-chi/transport v0.4.0 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.6.2 // indirect
	github.com/go-git/go-git/v5 v5.13.2 // indirect
	github.com/goccy/go-json v0.10.3 // indirect
	github.com/golang-cz/textcase v1.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/go-github v17.0.0+incompatible // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/goware/rerun v0.1.0 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/lestrrat-go/blackmagic v1.0.2 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/httprc v1.0.6 // indirect
	github.com/lestrrat-go/iter v1.0.2 // indirect
	github.com/lestrrat-go/jwx/v2 v2.1.3 // indirect
	github.com/lestrrat-go/option v1.0.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pjbgf/sha1cd v0.3.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/posener/diff v0.0.1 // indirect
	github.com/posener/gitfs v1.2.2 // indirect
	github.com/segmentio/asm v1.2.0 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/shurcooL/httpfs v0.0.0-20230704072500-f1e31cf0ba5c // indirect
	github.com/skeema/knownhosts v1.3.1 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/webrpc/gen-dart v0.1.1 // indirect
	github.com/webrpc/gen-golang v0.19.0 // indirect
	github.com/webrpc/gen-javascript v0.13.0 // indirect
	github.com/webrpc/gen-kotlin v0.1.0 // indirect
	github.com/webrpc/gen-openapi v0.16.3 // indirect
	github.com/webrpc/gen-typescript v0.17.0 // indirect
	github.com/webrpc/webrpc v0.26.0 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/mod v0.23.0 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sync v0.11.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/tools v0.30.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

tool (
	github.com/goware/rerun
	github.com/webrpc/webrpc
	github.com/webrpc/webrpc/cmd/webrpc-gen
)
