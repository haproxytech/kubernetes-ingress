#!/usr/bin/env bash                                                 
                                                                                                                                        
set -o errexit                                                                                                                          
set -o pipefail                                                                                                                         

command -v yq >/dev/null 2>&1 || { echo >&2 "yq not installed.  Aborting."; exit 1; }
command -v jq >/dev/null 2>&1 || { echo >&2 "jq not installed.  Aborting."; exit 1; }
                                                                    
CN_COMMIT=$(go list -m github.com/haproxytech/client-native/v2 | sed 's/^.*-//')

if [ -z "$1" ]; then echo >&2 "No model name supplied.  Aborting."; exit 1; fi
if [ -z "$CN_COMMIT" ]; then echo >&2 "Unable to get git commit for CN module.  Aborting."; exit 1; fi

curl -sk https://raw.githubusercontent.com/haproxytech/client-native/$CN_COMMIT/specification/models/configuration.yaml |
	yq |
  jq --arg MODEL $1 '.| 
  	reduce paths as $p(.;
    if $p[0] == $MODEL and $p[-1] == "$ref" then 
  		setpath($p[0:-1];getpath(getpath($p) | split("/")[-1]| split(" ")))
  	else 
  		. 
  	end
  	) | 
		.[$MODEL] |  
		walk(
				if type == "object" then with_entries( 
					if .key == "x-nullable" then 
						.key = "nullable" 
					elif (.key | contains("x-")) then 
						empty 
					else 
						. 
					end 
				) else . end
		)' |
  yq -y
