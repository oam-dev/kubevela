output: {
	apiVersion: "apps/v1"
	kind:       "Deployment"
	spec: {
		selector: matchLabels: {
			"app.oam.dev/component": context.name
			if parameter.addRevisionLabel {
				"app.oam.dev/appRevision": context.appRevision
			}
		}

		template: {
			metadata: labels: {
				"app.oam.dev/component": context.name
				if parameter.addRevisionLabel {
					"app.oam.dev/appRevision": context.appRevision
				}
			}

			spec: {
				containers: [{
					name:  context.name
					image: parameter.image

					if parameter["cmd"] != _|_ {
						command: parameter.cmd
					}

					if parameter["env"] != _|_ {
						env: parameter.env
					}

					if context["config"] != _|_ {
						env: context.config
					}

					ports: [{
						containerPort: parameter.port
					}]

					if parameter["cpu"] != _|_ {
						resources: {
							limits:
								cpu: parameter.cpu
							requests:
								cpu: parameter.cpu
						}
					}

					if parameter["volumes"] != _|_ {
						volumeMounts: [ for v in parameter.volumes {
							{
								mountPath: v.mountPath
								name:      v.name
							}}]
					}
				}]

			if parameter["volumes"] != _|_ {
				volumes: [ for v in parameter.volumes {
					{
						name: v.name
						if v.type == "pvc" {
							persistentVolumeClaim: {
								claimName: v.claimName
							}
						}
						if v.type == "configMap" {
							configMap: {
								defaultMode: v.defaultMode
								name:        v.cmName
								if v.items != _|_ {
									items: v.items
								}
							}
						}
						if v.type == "secret" {
							secret: {
								defaultMode: v.defaultMode
								secretName:  v.secretName
								if v.items != _|_ {
									items: v.items
								}
							}
						}
						if v.type == "emptyDir" {
							emptyDir: {
								medium: v.medium
							}
						}
					}}]
			}
		}
		}
	}
}
parameter: {
	// +usage=Which image would you like to use for your service
	// +short=i
	image: string

	// +usage=Commands to run in the container
	cmd?: [...string]

	// +usage=Which port do you want customer traffic sent to
	// +short=p
	port: *80 | int
	// +usage=Define arguments by using environment variables
	env?: [...{
		// +usage=Environment variable name
		name: string
		// +usage=The value of the environment variable
		value?: string
		// +usage=Specifies a source the value of this var should come from
		valueFrom?: {
			// +usage=Selects a key of a secret in the pod's namespace
			secretKeyRef: {
				// +usage=The name of the secret in the pod's namespace to select from
				name: string
				// +usage=The key of the secret to select from. Must be a valid secret key
				key: string
			}
		}
	}]
	// +usage=Number of CPU units for the service, like `0.5` (0.5 CPU core), `1` (1 CPU core)
	cpu?: string

	// +ignore If addRevisionLabel is true, the appRevision label will be added to the underlying pods
	addRevisionLabel: *false | bool

	// +usage=Declare volumes and volumeMounts
	volumes?: [...{
		name:      string
		mountPath: string
		// +usage=Specify volume type, options: "pvc","configMap","secret","emptyDir"
		type: "pvc" | "configMap" | "secret" | "emptyDir"
		if type == "pvc" {
			claimName: string
		}
		if type == "configMap" {
			defaultMode: *420 | int
			cmName:      string
			items?: [...{
				key:  string
				path: string
				mode: *511 | int
			}]
		}
		if type == "secret" {
			defaultMode: *420 | int
			secretName:  string
			items?: [...{
				key:  string
				path: string
				mode: *511 | int
			}]
		}
		if type == "emptyDir" {
			medium: *"" | "Memory"
		}
	}]
}
