#!/bin/bash

# 1. 设置环境变量进行交叉编译
# GOOS=linux 目标系统为 Linux
# GOARCH=amd64 目标架构为 Intel/AMD 64位
GOOS=linux GOARCH=amd64 go build -o main main.go

# 2. 腾讯云要求上传 zip 包
# 注意：zip 包里必须直接包含名为 main 的可执行文件
zip main.zip main

echo "编译完成，请上传 main.zip 到腾讯云函数控制台。"