// only place allowed to call time.Now, uuid.New*, or math/rand
// everywhere else goes through Clock, IDGenerator, or Rand — forbidigo enforces it
package platform
