# real_time_chat_app

用go+angular实现的一个简单的即时聊天demo

## 环境：

go:1.18

angular:v4

## 1.直接克隆下来

## 2.下载依赖就能跑了
go get github.com/gorilla/websocket

go get github.com/satori/go.uuid

## 3.前端部分在socketExample库里

## 4.前端部分用的angular
1.先安装Angular CLI

2.使用以下命令创建新项目

ng new SocketExample

3.进入SocketExample项目里面用命令行输入以下命令

ng g service socket

4.将以下文件改一下就行

src/app/socket.service.ts

src/app/app.module.ts 

src/app/app.component.ts

src/styles.css

src/app/app.component.html

## 5.前端直接克隆下来也能直接跑
