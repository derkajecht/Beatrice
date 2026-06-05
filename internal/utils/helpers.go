package utils

func ClearBuffer(buffer []byte) {
	for i := range buffer {
		buffer[i] = 0
	}
}
