package config

// 全局写入通道，传入待发送的帧数据
var WriteChan = make(chan []byte, 100)
