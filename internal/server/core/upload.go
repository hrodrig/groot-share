package core

func (c *Core) MaxUpload() int64 {
	if c.Cfg.MaxUploadBytes > 0 {
		return c.Cfg.MaxUploadBytes
	}
	return 32 << 30
}
