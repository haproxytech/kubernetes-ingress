#!/bin/bash

curl -X DELETE -H "Content-Type: application/yaml" --data-binary @configmap.yaml http://localhost:8081
curl -X DELETE -H "Content-Type: application/yaml" --data-binary @echo-app.yaml http://localhost:8081
curl -X DELETE -H "Content-Type: application/yaml" --data-binary @echo.endpointslices.yaml http://localhost:8081
