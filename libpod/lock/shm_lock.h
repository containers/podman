#ifndef shm_locks_h_
#define shm_locks_h_

#include <semaphore.h>
#include <stdint.h>

// Magic number to ensure we open the right SHM segment
#define MAGIC 0xA5A5

// Name of the SHM
#define SHM_NAME "/libpod_lock"

// Type for our bitmaps
typedef uint32_t bitmap_t;

// bitmap size
#define BITMAP_SIZE (sizeof(bitmap_t) * 8)

// Struct to hold a single bitmap and associated locks
typedef struct lock_group {
  bitmap_t bitmap;
  sem_t    locks[BITMAP_SIZE];
} lock_group_t;

// Struct to hold our SHM locks
typedef struct shm_struct {
  uint16_t     magic;
  sem_t        segment_lock;
  uint32_t     num_bitmaps;
  uint32_t     num_locks;
  lock_group_t locks[];
} shm_struct_t;

size_t compute_shm_size(uint32_t num_bitmaps);
shm_struct_t *setup_lock_shm(uint32_t num_locks, int *error_code);
shm_struct_t *open_lock_shm(uint32_t num_locks, int *error_code);
int32_t close_lock_shm(shm_struct_t *shm);
int64_t allocate_semaphore(shm_struct_t *shm);
int32_t deallocate_semaphore(shm_struct_t *shm, uint32_t sem_index);
int32_t lock_semaphore(shm_struct_t *shm, uint32_t sem_index);
int32_t unlock_semaphore(shm_struct_t *shm, uint32_t sem_index);

#endif
