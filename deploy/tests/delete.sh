#!/bin/bash
clustername=${1:-dev}
kind delete cluster --name $clustername
