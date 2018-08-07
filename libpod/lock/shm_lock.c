#include <errno.h>
#include <fcntl.h>
#include <semaphore.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>

#include "shm_lock.h"

// Compute the size of the SHM struct
size_t compute_shm_size(uint32_t num_bitmaps) {
  return sizeof(shm_struct_t) + (num_bitmaps * sizeof(lock_group_t));
}

// Set up an SHM segment holding locks for libpod.
// num_locks must be a multiple of BITMAP_SIZE (32 by default).
// Returns a valid pointer on success or NULL on error.
// If an error occurs, it will be written to the int pointed to by error_code.
shm_struct_t *setup_lock_shm(uint32_t num_locks, int *error_code) {
  int shm_fd, i, j, ret_code;
  uint32_t num_bitmaps;
  size_t shm_size;
  shm_struct_t *shm;

  // If error_code doesn't point to anything, we can't reasonably return errors
  // So fail immediately
  if (error_code == NULL) {
    return NULL;
  }

  // We need a nonzero number of locks
  if (num_locks == 0) {
    *error_code = EINVAL;
    return NULL;
  }

  // Calculate the number of bitmaps required
  if (num_locks % BITMAP_SIZE != 0) {
    // Number of locks not a multiple of BITMAP_SIZE
    *error_code = EINVAL;
    return NULL;
  }
  num_bitmaps = num_locks / BITMAP_SIZE;

  // Calculate size of the shm segment
  shm_size = compute_shm_size(num_bitmaps);

  // Create a new SHM segment for us
  shm_fd = shm_open(SHM_NAME, O_RDWR | O_CREAT | O_EXCL, 0600);
  if (shm_fd < 0) {
    *error_code = errno;
    return NULL;
  }

  // Increase its size to what we need
  ret_code = ftruncate(shm_fd, shm_size);
  if (ret_code < 0) {
    *error_code = errno;
    goto CLEANUP_UNLINK;
  }

  // Map the shared memory in
  shm = mmap(NULL, shm_size, PROT_READ | PROT_WRITE, MAP_SHARED, shm_fd, 0);
  if (shm == MAP_FAILED) {
    *error_code = errno;
    goto CLEANUP_UNLINK;
  }

  // We have successfully mapped the memory, now initialize the region
  shm->magic = MAGIC;
  shm->num_locks = num_locks;
  shm->num_bitmaps = num_bitmaps;

  // Initialize the semaphore that protects the bitmaps.
  // Initialize to value 1, as we're a mutex, and set pshared as this will be
  // shared between processes in an SHM.
  ret_code = sem_init(&(shm->segment_lock), true, 1);
  if (ret_code < 0) {
    *error_code = errno;
    goto CLEANUP_UNMAP;
  }

  // Initialize all bitmaps to 0 initially
  // And initialize all semaphores they use
  for (i = 0; i < num_bitmaps; i++) {
    shm->locks[i].bitmap = 0;
    for (j = 0; j < BITMAP_SIZE; j++) {
      // As above, initialize to 1 to act as a mutex, and set pshared as we'll
      // be living in an SHM.
      ret_code = sem_init(&(shm->locks[i].locks[j]), true, 1);
      if (ret_code < 0) {
	*error_code = errno;
	goto CLEANUP_UNMAP;
      }
    }
  }

  // Close the file descriptor, we're done with it
  // Ignore errors, it's ok if we leak a single FD and this should only run once
  close(shm_fd);

  return shm;

  // Cleanup after an error
 CLEANUP_UNMAP:
  munmap(shm, shm_size);
 CLEANUP_UNLINK:
  close(shm_fd);
  shm_unlink(SHM_NAME);
  return NULL;
}

// Open an existing SHM segment holding libpod locks.
// num_locks is the number of locks that will be configured in the SHM segment.
// num_locks must be a multiple of BITMAP_SIZE (32 by default).
// Returns a valid pointer on success or NULL on error.
// If an error occurs, it will be written to the int pointed to by error_code.
shm_struct_t *open_lock_shm(uint32_t num_locks, int *error_code) {
  int shm_fd;
  shm_struct_t *shm;
  size_t shm_size;
  uint32_t num_bitmaps;

  if (error_code == NULL) {
    return NULL;
  }

  // We need a nonzero number of locks
  if (num_locks == 0) {
    *error_code = EINVAL;
    return NULL;
  }

  // Calculate the number of bitmaps required
  if (num_locks % BITMAP_SIZE != 0) {
    // Number of locks not a multiple of BITMAP_SIZE
    *error_code = EINVAL;
    return NULL;
  }
  num_bitmaps = num_locks / BITMAP_SIZE;

  // Calculate size of the shm segment
  shm_size = compute_shm_size(num_bitmaps);

  shm_fd = shm_open(SHM_NAME, O_RDWR, 0600);
  if (shm_fd < 0) {
    return NULL;
  }

  // Map the shared memory in
  shm = mmap(NULL, shm_size, PROT_READ | PROT_WRITE, MAP_SHARED, shm_fd, 0);
  if (shm == MAP_FAILED) {
    *error_code = errno;
  }

  // Ignore errors, it's ok if we leak a single FD since this only runs once
  close(shm_fd);

  // Check if we successfully mmap'd
  if (shm == MAP_FAILED) {
    return NULL;
  }

  // Need to check the SHM to see if it's actually our locks
  if (shm->magic != MAGIC) {
    *error_code = errno;
    goto CLEANUP;
  }
  if (shm->num_locks != num_locks) {
    *error_code = errno;
    goto CLEANUP;
  }

  return shm;

 CLEANUP:
  munmap(shm, shm_size);
  return NULL;
}

// Close an open SHM lock struct, unmapping the backing memory.
// The given shm_struct_t will be rendered unusable as a result.
// On success, 0 is returned. On failure, negative ERRNO values are returned.
int32_t close_lock_shm(shm_struct_t *shm) {
  int ret_code;
  size_t shm_size;

  // We can't unmap null...
  if (shm == NULL) {
    return -1 * EINVAL;
  }

  shm_size = compute_shm_size(shm->num_bitmaps);

  ret_code = munmap(shm, shm_size);

  if (ret_code != 0) {
    return -1 * errno;
  }

  return 0;
}

// Allocate the first available semaphore
// Returns a positive integer guaranteed to be less than UINT32_MAX on success,
// or negative errno values on failure
// On sucess, the returned integer is the number of the semaphore allocated
int64_t allocate_semaphore(shm_struct_t *shm) {
  int ret_code, i;
  bitmap_t test_map;
  int64_t sem_number, num_within_bitmap;

  if (shm == NULL) {
    return -1 * EINVAL;
  }

  // Lock the semaphore controlling access to our shared memory
  do {
    ret_code = sem_wait(&(shm->segment_lock));
  } while(ret_code == EINTR);
  if (ret_code != 0) {
    return -1 * errno;
  }

  // Loop through our bitmaps to search for one that is not full
  for (i = 0; i < shm->num_bitmaps; i++) {
    if (shm->locks[i].bitmap != 0xFFFFFFFF) {
      test_map = 0x1;
      num_within_bitmap = 0;
      while (test_map != 0) {
	if ((test_map & shm->locks[i].bitmap) == 0) {
	  // Compute the number of the semaphore we are allocating
	  sem_number = (BITMAP_SIZE * i) + num_within_bitmap;
	  // OR in the bitmap
	  shm->locks[i].bitmap = shm->locks[i].bitmap | test_map;
	  // Clear the semaphore
	  sem_post(&(shm->segment_lock));
	  // Return the semaphore we've allocated
	  return sem_number;
	}
	test_map = test_map << 1;
	num_within_bitmap++;
      }
      // We should never fall through this loop
      // TODO maybe an assert() here to panic if we do?
    }
  }

  // Post to the semaphore to clear the lock
  sem_post(&(shm->segment_lock));

  // All bitmaps are full
  // We have no available semaphores, report allocation failure
  return -1 * ENOSPC;
}

// Deallocate a given semaphore
// Returns 0 on success, negative ERRNO values on failure
int32_t deallocate_semaphore(shm_struct_t *shm, uint32_t sem_index) {
  bitmap_t test_map;
  int bitmap_index, index_in_bitmap, ret_code, i;

  if (shm == NULL) {
    return -1 * EINVAL;
  }

  // Check if the lock index is valid
  if (sem_index >= shm->num_locks) {
    return -1 * EINVAL;
  }

  bitmap_index = sem_index / BITMAP_SIZE;
  index_in_bitmap = sem_index % BITMAP_SIZE;

  // This should never happen if the sem_index test above succeeded, but better
  // safe than sorry
  if (bitmap_index >= shm->num_bitmaps) {
    return -1 * EFAULT;
  }

  test_map = 0x1;
  for (i = 0; i < index_in_bitmap; i++) {
    test_map = test_map << 1;
  }

  // Lock the semaphore controlling access to our shared memory
  do {
    ret_code = sem_wait(&(shm->segment_lock));
  } while(ret_code == EINTR);
  if (ret_code != 0) {
    return -1 * errno;
  }

  // Check if the semaphore is allocated
  if ((test_map & shm->locks[bitmap_index].bitmap) == 0) {
    // Post to the semaphore to clear the lock
    sem_post(&(shm->segment_lock));

    return -1 * ENOENT;
  }

  // The semaphore is allocated, clear it
  // Invert the bitmask we used to test to clear the bit
  test_map = ~test_map;
  shm->locks[bitmap_index].bitmap = shm->locks[bitmap_index].bitmap & test_map;

  // Post to the semaphore to clear the lock
  sem_post(&(shm->segment_lock));

  return 0;
}

// Lock a given semaphore
// Does not check if the semaphore is allocated - this ensures that, even for
// removed containers, we can still successfully lock to check status (and
// subsequently realize they have been removed).
// Returns 0 on success, -1 on failure
int32_t lock_semaphore(shm_struct_t *shm, uint32_t sem_index) {
  int bitmap_index, index_in_bitmap, ret_code;

  if (shm == NULL) {
    return -1 * EINVAL;
  }

  if (sem_index >= shm->num_locks) {
    return -1 * EINVAL;
  }

  bitmap_index = sem_index / BITMAP_SIZE;
  index_in_bitmap = sem_index % BITMAP_SIZE;

  // Lock the semaphore controlling access to our shared memory
  do {
    ret_code = sem_wait(&(shm->locks[bitmap_index].locks[index_in_bitmap]));
  } while(ret_code == EINTR);
  if (ret_code != 0) {
    return -1 * errno;
  }

  return 0;
}

// Unlock a given semaphore
// Does not check if the semaphore is allocated - this ensures that, even for
// removed containers, we can still successfully lock to check status (and
// subsequently realize they have been removed).
// Returns 0 on success, -1 on failure
int32_t unlock_semaphore(shm_struct_t *shm, uint32_t sem_index) {
  int bitmap_index, index_in_bitmap, ret_code;
  unsigned int sem_value = 0;

  if (shm == NULL) {
    return -1 * EINVAL;
  }

  if (sem_index >= shm->num_locks) {
    return -1 * EINVAL;
  }

  bitmap_index = sem_index / BITMAP_SIZE;
  index_in_bitmap = sem_index % BITMAP_SIZE;

  // Only allow a post if the semaphore is less than 1 (locked)
  // This allows us to preserve mutex behavior
  ret_code = sem_getvalue(&(shm->locks[bitmap_index].locks[index_in_bitmap]), &sem_value);
  if (ret_code != 0) {
    return -1 * errno;
  }
  if (sem_value >= 1) {
    return -1 * EBUSY;
  }

  ret_code = sem_post(&(shm->locks[bitmap_index].locks[index_in_bitmap]));
  if (ret_code != 0) {
    return -1 * errno;
  }

  return 0;
}
