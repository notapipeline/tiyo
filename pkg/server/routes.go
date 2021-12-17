// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package server

func (server *Server) setupRoutes(bfs *BinFileSystem) {
	server.router.Use(server.RequireAccount)

	server.engine.GET("/configure", server.Configure)
	server.engine.POST("/configure", server.Configure)

	// page methods
	server.router.GET("/", server.Index)
	server.router.GET("/pipeline", server.Index)
	server.router.GET("/scan", server.Index)
	server.router.GET("/scan/:bucket", server.Index)
	server.router.GET("/buckets", server.Index)
	server.router.GET("/login", server.Signin)
	server.router.POST("/login", server.Signin)
	server.router.GET("/logout", server.Signout)

	// api methods
	server.engine.GET("/api/v1/bucket", server.api.Buckets)
	server.engine.GET("/api/v1/bucket/:bucket/:child", server.api.Get)
	server.engine.GET("/api/v1/bucket/:bucket/:child/*key", server.api.Get)

	server.engine.PUT("/api/v1/bucket", server.api.Put)
	server.engine.POST("/api/v1/bucket", server.api.CreateBucket)

	server.engine.DELETE("/api/v1/bucket/:bucket", server.api.DeleteBucket)
	server.engine.DELETE("/api/v1/bucket/:bucket/:child", server.api.DeleteKey)
	server.engine.DELETE("/api/v1/bucket/:bucket/:child/*key", server.api.DeleteKey)

	server.engine.GET("/api/v1/containers", server.api.Containers)
	server.engine.GET("/api/v1/collections/:collection", bfs.Collection)

	server.engine.GET("/api/v1/scan/:bucket", server.api.PrefixScan)
	server.engine.GET("/api/v1/scan/:bucket/:child", server.api.PrefixScan)
	server.engine.GET("/api/v1/scan/:bucket/:child/*key", server.api.PrefixScan)

	server.engine.GET("/api/v1/count/:bucket", server.api.KeyCount)
	server.engine.GET("/api/v1/count/:bucket/*child", server.api.KeyCount)

	server.engine.GET("/api/v1/popqueue/:pipeline/:key", server.api.PopQueue)
	server.engine.POST("/api/v1/perpetualqueue", server.api.PerpetualQueue)

	server.engine.GET("/api/v1/status/:pipeline", server.api.FlowStatus)
	server.engine.POST("/api/v1/execute", server.api.ExecuteFlow)
	server.engine.POST("/api/v1/startflow", server.api.StartFlow)
	server.engine.POST("/api/v1/stopflow", server.api.StopFlow)
	server.engine.POST("/api/v1/destroyflow", server.api.DestroyFlow)
	server.engine.POST("/api/v1/encrypt", server.api.Encrypt)
	server.engine.POST("/api/v1/decrypt", server.api.Decrypt)

	server.engine.POST("/addmachine", server.addmachine)
	server.engine.GET("/hmac", server.hmac)
}
