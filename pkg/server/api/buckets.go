package api

import (
	"bytes"
	"fmt"
	"strings"

	"net/http"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// Basic bucket type
type bucket struct {
	Bucket string `json:"bucket" uri:"bucket" binding:"required"`
	Child  string `json:"child,omitempty" uri:"child,omitempty"`
	Key    string `json:"key,omitempty" uri:"key,omitempty"`
	Value  string `json:"value,omitempty"`
}

// Buckets : List all available top level buckets
//
// Returned responses will be one of:
// - 200 OK    : Message will be a list of bucket names
// - 500 Error : Message will be the error message
func (api *API) Buckets(c *gin.Context) {
	result := NewResult()
	result.Code = 200
	result.Result = "OK"
	result.Message = make([]string, 0)
	if err := api.Db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
			b := []string{string(name)}
			result.Message = append(
				result.Message.([]string), b...)
			return nil
		})
	}); err != nil {
		result = NewResult()
		result.Code = 500
		result.Result = "Error"
		result.Message = fmt.Sprintf("%s", err)
	}
	c.JSON(result.Code, result)
}

// CreateBucket : Create a new bucket
//
// POST /bucket
//
// The create request must contain:
// - bucket - the name of the bucket to create or parent bucket name when creating a child
// - child  - [optional] the name of the child bucket to create
//
// Responses are:
// - 201 Created
// - 400 Bad request sent when bucket name is empty
// - 500 Internal server error
func (api *API) CreateBucket(c *gin.Context) {
	result := NewResult()
	result.Code = 201
	result.Result = "OK"

	request := bucket{}
	if err := api.bind(c, &request); err != nil {
		c.JSON(err.Code, err)
		return
	}

	if err := api.Db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(request.Bucket))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		if request.Child != "" {
			if _, err = bucket.CreateBucketIfNotExists([]byte(request.Child)); err != nil {
				return fmt.Errorf("Error creating child bucket %s", request.Child)
			}
		}
		return nil
	}); err != nil {
		result.Code = 500
		result.Result = "Error"
		result.Message = fmt.Sprintf("%s", err)
	}
	c.JSON(result.Code, result)
}

// DeleteBucket : Deletes a given bucket
//
// DELETE /bucket/:bucket[/:child]
//
// The url parameters must contain:
// - bucket - the name of the bucket to delete or parent bucket name when deleting a child
// - child  - [optional] the name of the child bucket to create
//
// Responses are:
// - 202 Accepted
// - 400 Bad request sent when bucket name is empty
// - 500 Internal server error
func (api *API) DeleteBucket(c *gin.Context) {
	result := NewResult()
	result.Code = 202
	result.Result = "OK"

	request := bucket{}
	if err := api.bind(c, &request); err != nil {
		c.JSON(result.Code, err)
		return
	}

	if err := api.Db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket([]byte(request.Bucket))
		return err
	}); err != nil {
		result.Code = 500
		result.Result = "Error"
		result.Message = fmt.Sprintf("%s", err)
	}
	if result.Code == 202 {
		c.JSON(result.Code, nil)
	} else {
		c.JSON(result.Code, result)
	}

}

// DeleteKey : Delete a key value pair from a bucket
//
// DELETE /bucket/:bucket[/:child]/:key
//
// URL parameters are:
// - bucket : The name of the bucket containing the key
// - child  : [optional] The name of a child bucket to manage
// - key    : The key to delete
//
// Returned responses are:
// - 202 Accepted
// - 400 Bad Request when bucket or key are empty
// - 500 Internal server error on failure to delete
func (api *API) DeleteKey(c *gin.Context) {
	result := NewResult()
	result.Code = 202
	result.Result = "OK"

	request := bucket{}
	if err := api.bind(c, &request); err != nil {
		c.JSON(err.Code, err)
		return
	}

	if err := api.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(request.Bucket))
		if b == nil {
			return fmt.Errorf("No such bucket")
		}

		if request.Child != "" {
			b = b.Bucket([]byte(request.Child))
		}

		if val := b.Get([]byte(request.Key)); val == nil {
			if err := b.DeleteBucket([]byte(request.Key)); err != nil {
				return fmt.Errorf("Error deleting inner bucket %s - %s", request.Key, err)
			}
		}

		if err := b.Delete([]byte(request.Key)); err != nil {
			return fmt.Errorf("Error deleting key %s - %s", request.Key, err)
		}
		return nil
	}); err != nil {
		log.Println(err)
		result.Code = 500
		result.Result = "Error"
		result.Message = err
	}
	if result.Code == 202 {
		c.JSON(result.Code, nil)
	} else {
		c.JSON(result.Code, result)
	}

}

// Put : create a key/value pair in the boltdb
//
// PUT /bucket
//
// Request parameters:
// - bucket The bucket to write into
// - child  [optional] the child bucket to write into
// - key    The key to add
// - value  The value to save against the key
//
// Response codes
// - 204 No content if key successfully stored
// - 400 Bad request if bucket or key is missing
// - 500 Internal server error if value cannot be stored
func (api *API) Put(c *gin.Context) {
	result := NewResult()
	result.Code = 204
	result.Result = "OK"

	request := bucket{}
	if err := api.bind(c, &request); err != nil {
		c.JSON(err.Code, err)
		return
	}

	if err := api.Db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(request.Bucket))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}

		if request.Child != "" {
			b = b.Bucket([]byte(request.Child))
		}

		//log.Debug(request, b)
		err = b.Put([]byte(request.Key), []byte(request.Value))
		if err != nil {
			return fmt.Errorf("create kv: %s", err)
		}

		return nil
	}); err != nil {
		result.Code = 500
		result.Result = "Error"
		result.Message = err
	}
	if result.Code == 204 {
		c.JSON(result.Code, nil)
	} else {
		c.JSON(result.Code, result)
	}

}

// Get : Get the value for a given key
//
// GET /bucket/:bucket[/:child]/:key
//
// Response codes
// - 200 OK - Message will be the value
// - 400 Bad Request if bucket or key is empty
// - 500 Internal server error if value cannot be retrieved
func (api *API) Get(c *gin.Context) {
	result := NewResult()
	result.Code = 200
	result.Result = "OK"

	request := bucket{}
	if err := api.bind(c, &request); err != nil {
		c.JSON(err.Code, err)
		return
	}

	if err := api.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(request.Bucket))
		if b == nil {
			return fmt.Errorf("No such bucket")
		}

		if request.Child != "" {
			b = b.Bucket([]byte(request.Child))
		}

		value := b.Get([]byte(request.Key))
		if value == nil {
			return fmt.Errorf("Key not found")
		}
		result.Message = string(value)
		return nil
	}); err != nil {
		result.Code = 500
		result.Result = "Error"
		result.Message = err
	}
	c.JSON(result.Code, result)

}

// PrefixScan : Scan a bucket and retrieve all contents with keys matching prefix
//
// GET /scan/:bucket[/:child][/:key]
//
// Request parameters
// - bucket [required] The bucket name to scan
// - child  [optional] The child bucket to scan instead
// - key    [optional] If key is not specified, all contents will be returned
//
// Response codes
// - 200 OK Messsage will be a map of matching key/value pairs
// - 400 Bad Request if bucket name is empty
// - 500 Internal server error if the request fails for any reason
func (api *API) PrefixScan(c *gin.Context) {
	result := Result{Result: "error"}
	result.Code = 200
	result.Result = "OK"
	result.Message = make(map[string]interface{})

	request := bucket{}
	if err := api.bind(c, &request); err != nil {
		c.JSON(err.Code, err)
		return
	}

	scanResults := ScanResult{}
	scanResults.Buckets = make([]string, 0)
	scanResults.BucketsLength = 0
	scanResults.Keys = make(map[string]string)
	scanResults.KeyLen = 0

	if err := api.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(request.Bucket))

		if request.Child != "" {
			b = b.Bucket([]byte(request.Child))
		}
		if b == nil {
			return fmt.Errorf("No such bucket or bucket is invalid")
		}
		c := b.Cursor()

		if request.Key != "" {
			prefix := []byte(request.Key)
			for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
				if v == nil {
					scanResults.Buckets = append(scanResults.Buckets, string(k))
					scanResults.BucketsLength++
				} else {
					scanResults.Keys[string(k)] = string(v)
					scanResults.KeyLen++
				}
			}
		} else {
			for k, v := c.First(); k != nil; k, v = c.Next() {
				if v == nil {
					scanResults.Buckets = append(scanResults.Buckets, string(k))
					scanResults.BucketsLength++
				} else {
					scanResults.Keys[string(k)] = string(v)
					scanResults.KeyLen++
				}
			}
		}
		return nil
	}); err != nil {
		log.Error(err)
		result.Code = 500
		result.Result = "Error"
		result.Message = err.Error()
	}
	result.Message = scanResults
	c.JSON(result.Code, result)

}

// KeyCount : Count keys in a bucket
//
// GET /count/:bucket[/:child]
//
// Response codes:
// - 200 OK - Message will be the number of keys in the bucket
// - 400 Bad Request - Bucket name is missing
// - 404 Page not found - Bucket name is invalid
// - 500 Internal server error on all other errors
func (api *API) KeyCount(c *gin.Context) {
	result := Result{Result: "error"}
	result.Code = 200
	result.Result = "OK"

	request := bucket{}
	if err := api.bind(c, &request); err != nil {
		c.JSON(err.Code, err)
		return
	}

	var count int = 0
	if err := api.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(request.Bucket))
		if b != nil {
			if request.Child != "" {
				b = b.Bucket([]byte(request.Child))
			}
			c := b.Cursor()
			for k, _ := c.First(); k != nil; k, _ = c.Next() {
				count++
			}
			result.Message = count
		} else {
			result.Code = 404
			result.Result = "Error"
			result.Message = "no such bucket available"
		}
		return nil
	}); err != nil {
		result.Code = 500
		result.Result = "Error"
		result.Message = err
	}
	c.JSON(result.Code, result)
}

func (api *API) bind(c *gin.Context, request *bucket) *Result {
	result := Result{Result: "error"}
	result.Code = 400
	result.Result = "Error"

	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodDelete {
		if err := c.ShouldBindUri(&request); err != nil {
			log.Error(err)
			result.Message = err.Error()
			return &result
		}
	} else {
		if err := c.ShouldBindJSON(&request); err != nil {
			log.Error(err)
			result.Message = err.Error()
			return &result
		}
	}

	if request.Key == "" && request.Child != "" {
		request.Key = request.Child
		request.Child = ""
	}
	request.Key = strings.Trim(request.Key, "/")

	return nil
}
