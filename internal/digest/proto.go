package digest

import repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"

// FromProto validates a protocol Digest and converts it.
func FromProto(p *repb.Digest) (Digest, error) {
	return New(p.GetHash(), p.GetSizeBytes())
}

// ToProto converts d to its protocol form.
func (d Digest) ToProto() *repb.Digest {
	return &repb.Digest{Hash: d.Hash, SizeBytes: d.Size}
}
