package ledger

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

const alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

func NewID(now time.Time) string {
	return newIDWithPrefix("K", now)
}

func NewRecurringID(now time.Time) string {
	return newIDWithPrefix("R", now)
}

func newIDWithPrefix(prefix string, now time.Time) string {
	buf := make([]byte, 4)
	for i := range buf {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			buf[i] = alphabet[now.UnixNano()%int64(len(alphabet))]
			continue
		}
		buf[i] = alphabet[n.Int64()]
	}
	return fmt.Sprintf("%s-%s-%s", prefix, now.Format("20060102"), string(buf))
}
