// Command `formspec sign` — ed25519 module signing (todo 13.3.6,
// 07-marketplace.md §2 trust tiers).
//
//	formspec sign keygen --out <dir> --name <vendor>
//	formspec sign <module-dir> --key <private.key> [--out <sig-file>]
//	formspec sign verify <module-dir> --signature <b64|file> --public-key <pub.file>
//
// The signed payload is the module's tree checksum (internal/vendor
// TreeChecksum) — the same value recorded in formspec.lock and the registry.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/primadi/formspec/internal/vendor"
)

func runSign(args []string) {
	if len(args) < 1 {
		usageSign()
		os.Exit(2)
	}
	if args[0] == "keygen" {
		runSignKeygen(args[1:])
		return
	}
	if args[0] == "verify" {
		runSignVerify(args[1:])
		return
	}
	// `formspec sign <module-dir>` — sign the tree checksum.
	runSignModule(args)
}

func usageSign() {
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  formspec sign keygen --out <dir> --name <vendor>\n")
	fmt.Fprintf(os.Stderr, "  formspec sign <module-dir> --key <private.key> [--out <sig-file>]\n")
	fmt.Fprintf(os.Stderr, "  formspec sign verify <module-dir> --signature <b64|file> --public-key <pub.file>\n")
}

func runSignKeygen(args []string) {
	fs := flag.NewFlagSet("sign keygen", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("out", ".", "output directory")
	name := fs.String("name", "vendor", "key file base name")
	fs.Parse(args)

	kp, err := vendor.GenerateKeyPair()
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec sign keygen: %v\n", err)
		os.Exit(1)
	}
	privPath, pubPath, err := vendor.SaveKeyPair(*out, *name, kp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec sign keygen: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("keypair generated:\n")
	fmt.Printf("  private: %s (JANGAN di-commit — setara password vendor)\n", privPath)
	fmt.Printf("  public:  %s (dibagikan / didaftarkan ke registry vendor)\n", pubPath)
}

func runSignModule(args []string) {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	key := fs.String("key", "", "private key file (base64)")
	out := fs.String("out", "", "write signature to file instead of stdout")
	positional := reorderFlags(fs, args, nil)
	if len(positional) < 1 || *key == "" {
		usageSign()
		os.Exit(2)
	}

	checksum, err := vendor.TreeChecksum(positional[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec sign: %v\n", err)
		os.Exit(1)
	}
	priv, err := vendor.LoadKeyFile(*key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec sign: %v\n", err)
		os.Exit(1)
	}
	sig, err := vendor.SignChecksum(priv, checksum)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec sign: %v\n", err)
		os.Exit(1)
	}
	if *out != "" {
		if err := os.WriteFile(*out, []byte(sig+"\n"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "formspec sign: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("signature written to %s\n", *out)
	} else {
		fmt.Println(sig)
	}
	fmt.Fprintf(os.Stderr, "checksum: %s\n", checksum)
}

func runSignVerify(args []string) {
	fs := flag.NewFlagSet("sign verify", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	sig := fs.String("signature", "", "base64 signature, or a file path")
	pub := fs.String("public-key", "", "public key file (base64)")
	positional := reorderFlags(fs, args, nil)
	if len(positional) < 1 || *sig == "" || *pub == "" {
		usageSign()
		os.Exit(2)
	}

	checksum, err := vendor.TreeChecksum(positional[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec sign verify: %v\n", err)
		os.Exit(1)
	}
	signature := *sig
	if data, err := os.ReadFile(*sig); err == nil {
		signature = strings.TrimSpace(string(data))
	}
	publicKey, err := vendor.LoadKeyFile(*pub)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec sign verify: %v\n", err)
		os.Exit(1)
	}
	if err := vendor.VerifyChecksum(publicKey, checksum, signature); err != nil {
		fmt.Fprintf(os.Stderr, "formspec sign verify: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("signature OK — %s\n", checksum)
}
