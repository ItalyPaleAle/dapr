{{/*
Returns the address and port of the emmy service
The returned value is a string in the format "<name>:<port>"
*/}}
{{- define "address.emmy" -}}
{{- "dapr-emmy:51101" }}
{{- end -}}
