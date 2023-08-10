module brbot

go 1.20

replace github.com/companyzero/bisonrelay-bot/bot => ../bot

require (
	github.com/companyzero/bisonrelay v0.1.8
	github.com/companyzero/bisonrelay-bot/bot v0.0.0-00010101000000-000000000000
	github.com/decred/go-socks v1.1.0
	github.com/decred/slog v1.2.0
	github.com/jrick/logrotate v1.0.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412 // indirect
	github.com/companyzero/sntrup4591761 v0.0.0-20220309191932-9e0f3af2f07a // indirect
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/decred/base58 v1.0.5 // indirect
	github.com/decred/dcrd/chaincfg/chainhash v1.0.4 // indirect
	github.com/decred/dcrd/crypto/blake256 v1.0.1 // indirect
	github.com/decred/dcrd/crypto/ripemd160 v1.0.2 // indirect
	github.com/decred/dcrd/dcrec v1.0.1 // indirect
	github.com/decred/dcrd/dcrec/edwards/v2 v2.0.3 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.2.0 // indirect
	github.com/decred/dcrd/dcrutil/v4 v4.0.1 // indirect
	github.com/decred/dcrd/txscript/v4 v4.1.0 // indirect
	github.com/decred/dcrd/wire v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	golang.org/x/crypto v0.8.0 // indirect
	golang.org/x/sync v0.3.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	lukechampine.com/blake3 v1.2.1 // indirect
)
