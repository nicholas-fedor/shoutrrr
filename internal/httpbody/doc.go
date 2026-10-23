// Package httpbody reads HTTP response bodies up to a size limit.
//
// Services read the bodies of responses from servers they do not control. A
// server that answers with an endless or very large body would otherwise make a
// service buffer all of it. Reading through this package stops one byte past
// the limit and reports an oversized body as [ErrTooLarge].
package httpbody
