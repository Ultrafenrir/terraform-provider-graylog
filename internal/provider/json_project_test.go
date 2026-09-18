package provider

import "testing"

func TestProjectAndCanonicalizeJSON(t *testing.T) {
	cases := []struct {
		name   string
		server string
		mask   string
		want   string
	}{
		{
			// The exact shape Graylog returns for a guava_cache: the four
			// null-valued keys were never submitted by the practitioner.
			name: "server defaults outside the mask are dropped",
			server: `{"type":"guava_cache","max_size":1000,"expire_after_access":60,
			          "expire_after_access_unit":"SECONDS","expire_after_write":0,
			          "expire_after_write_unit":null,"ignore_null":null,
			          "ttl_empty":null,"ttl_empty_unit":null}`,
			mask: `{"type":"guava_cache","max_size":1000,"expire_after_access":60,
			        "expire_after_access_unit":"SECONDS","expire_after_write":0}`,
			want: `{"expire_after_access":60,"expire_after_access_unit":"SECONDS","expire_after_write":0,"max_size":1000,"type":"guava_cache"}`,
		},
		{
			name:   "a managed key changed server-side survives projection",
			server: `{"max_size":500,"ignore_null":null}`,
			mask:   `{"max_size":1000}`,
			want:   `{"max_size":500}`,
		},
		{
			name:   "a managed key dropped server-side is absent from the projection",
			server: `{"other":1}`,
			mask:   `{"max_size":1000}`,
			want:   `{}`,
		},
		{
			name:   "projection recurses into nested objects",
			server: `{"outer":{"kept":1,"added":2},"top":3}`,
			mask:   `{"outer":{"kept":9}}`,
			want:   `{"outer":{"kept":1}}`,
		},
		{
			// A different length is membership drift and remains visible.
			name:   "array membership changes are taken whole",
			server: `{"names":["a","b","c"]}`,
			mask:   `{"names":["a"]}`,
			want:   `{"names":["a","b","c"]}`,
		},
		{
			name:   "server defaults inside array objects are dropped",
			server: `{"series":[{"id":"count","function":"count","type":null}]}`,
			mask:   `{"series":[{"id":"count","function":"count"}]}`,
			want:   `{"series":[{"function":"count","id":"count"}]}`,
		},
		{
			name:   "large integers keep their notation",
			server: `{"threshold":20000000,"extra":null}`,
			mask:   `{"threshold":1}`,
			want:   `{"threshold":20000000}`,
		},
		{
			// An imported resource has no document in state yet.
			name:   "an empty mask yields the whole canonical document",
			server: `{"b":2,"a":1}`,
			mask:   ``,
			want:   `{"a":1,"b":2}`,
		},
		{
			name:   "key order is irrelevant",
			server: `{"b":2,"a":1}`,
			mask:   `{"a":0,"b":0}`,
			want:   `{"a":1,"b":2}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ProjectAndCanonicalizeJSON(tc.server, tc.mask)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("projection mismatch\n want: %s\n  got: %s", tc.want, got)
			}
		})
	}
}

// The projection is what keeps a plan empty when nothing changed: the
// practitioner's own document, projected onto the enriched server echo, must
// come back canonically identical to itself.
func TestProjectAndCanonicalizeJSON_IdempotentForUnchangedConfig(t *testing.T) {
	practitioner := `{"type":"guava_cache","max_size":1000,"expire_after_access":60}`
	serverEcho := `{"type":"guava_cache","max_size":1000,"expire_after_access":60,
	                "expire_after_access_unit":null,"ignore_null":null}`

	projected, err := ProjectAndCanonicalizeJSON(serverEcho, practitioner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	canonicalPractitioner, err := CanonicalizeJSONFromString(practitioner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projected != canonicalPractitioner {
		t.Fatalf("unchanged configuration would show a diff\n config: %s\n server: %s",
			canonicalPractitioner, projected)
	}
}

func TestProjectAndCanonicalizeJSON_InvalidServerDocument(t *testing.T) {
	if _, err := ProjectAndCanonicalizeJSON(`{not json`, `{}`); err == nil {
		t.Fatal("expected an error for an unparseable server document")
	}
}
