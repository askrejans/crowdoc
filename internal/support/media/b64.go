package media

import "encoding/base64"

var encodings = []*base64.Encoding{
	base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding,
}
