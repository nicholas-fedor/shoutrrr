// Package sequential makes the requests of one send one after another, such
// as one request per chat, recipient, or message batch.
//
// A send keeps going after a request fails, so one failure does not cost the
// others their message. Once the send's context ends, every remaining request
// would fail with the context's error. The send then stops and reports that
// error once. A [Policy] decides how long each request may take.
package sequential
