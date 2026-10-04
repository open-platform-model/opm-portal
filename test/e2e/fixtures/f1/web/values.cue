// Starting values from the module's debugValues, the author's test values.
// Review them before deploying.
package instance

values: {
	image: {
		repository: "nginx"
		tag:        "1.27"
		// Pinned by digest so a moved 1.27 tag cannot change the capture.
		digest: "sha256:6784fb0834aa7dbbe12e3d7471e69c290df3e6ba810dc38b34ae33d3c1c05f7d"
	}
	replicas:    1
	port:        80
	serviceType: "ClusterIP"
}
