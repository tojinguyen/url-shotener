package util

import "crypto/md5"

// MD5 returns the raw 16-byte MD5 digest of s.
func MD5(s string) []byte {
	sum := md5.Sum([]byte(s))
	return sum[:]
}
