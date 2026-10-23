package dnsinspect

import "context"

func (ins *inspector) encryptedCheck(ctx context.Context, r Request) []EncryptedResult {
	out := make([]EncryptedResult, 0, len(r.DoH)+len(r.DoT))
	for _, endpoint := range r.DoH {
		out = append(out, ins.doh(ctx, endpoint, r))
	}
	for _, endpoint := range r.DoT {
		out = append(out, ins.dot(ctx, endpoint, r))
	}
	return out
}
