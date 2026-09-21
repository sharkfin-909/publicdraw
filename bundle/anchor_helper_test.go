package bundle_test

import "encoding/base64"

func toB64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
