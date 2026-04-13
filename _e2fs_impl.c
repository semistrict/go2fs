// e2fs.c — thin C wrapper around libext2fs for creating ext4 images from Go.
//
// This file is compiled by CGo. It links against the static libraries built
// from the e2fsprogs submodule in third_party/e2fsprogs.

// go2fs: compiled via ccgo (not CGo). Adjusted includes for ccgo build.
#include "config.h"
#include "_e2fs_impl.h"

#include <string.h>
#include <stdlib.h>
#include <time.h>
#include <errno.h>

#include "ext2fs/ext2fs.h"
#include "et/com_err.h"
#include "e2p/e2p.h"
#include "uuid/uuid.h"

// go2fs: fill buffer with pseudo-random bytes.
// uuid_generate() and getentropy() are not available in modernc/libc
// on darwin/arm64. This uses a simple PRNG seeded with time —
// not cryptographically secure but sufficient for filesystem UUIDs.
static void e2fs_fill_random(void *buf, size_t len) {
	unsigned char *p = (unsigned char *)buf;
	unsigned long seed = (unsigned long)time(NULL) ^ (unsigned long)(uintptr_t)buf;
	for (size_t i = 0; i < len; i++) {
		seed = seed * 6364136223846793005ULL + 1442695040888963407ULL;
		p[i] = (unsigned char)(seed >> 33);
	}
}

// Minimal dirname/basename — avoids libgen.h which ccgo may not support.
static char *_e2fs_dirname(char *path) {
	char *last = strrchr(path, '/');
	if (!last) return ".";  // no slash -> "."
	if (last == path) return "/";
	*last = '\0';
	return path;
}
static char *_e2fs_basename(char *path) {
	char *last = strrchr(path, '/');
	return last ? last + 1 : path;
}
#define dirname _e2fs_dirname
#define basename _e2fs_basename

// Features matching mkfsExt4Features in ext4_common.go.
#define MKFS_FEATURES \
	"has_journal,ext_attr,resize_inode,dir_index,filetype," \
	"extent,64bit,flex_bg,sparse_super,large_file,huge_file," \
	"dir_nlink,extra_isize,metadata_csum"

// ---- helpers ----

// Resolve a directory path, creating intermediate directories as needed.
// Returns the inode of the final component.
static errcode_t find_or_mkdir_path(ext2_filsys fs, ext2_ino_t root,
				    const char *path, ext2_ino_t *out)
{
	errcode_t ret;
	ext2_ino_t cur = root, next;
	char *dup = strdup(path);
	if (!dup)
		return ENOMEM;

	char *p = dup;
	// skip leading slashes
	while (*p == '/')
		p++;

	while (*p) {
		char *slash = strchr(p, '/');
		if (slash)
			*slash = '\0';

		if (*p == '\0' || (p[0] == '.' && p[1] == '\0')) {
			// skip empty or "." components
			if (slash) {
				p = slash + 1;
				continue;
			}
			break;
		}

		ret = ext2fs_lookup(fs, cur, p, strlen(p), 0, &next);
		if (ret == EXT2_ET_FILE_NOT_FOUND) {
			// create the directory
			ret = ext2fs_mkdir(fs, cur, 0, p);
			if (ret) {
				free(dup);
				return ret;
			}
			ret = ext2fs_lookup(fs, cur, p, strlen(p), 0, &next);
		}
		if (ret) {
			free(dup);
			return ret;
		}
		cur = next;

		if (slash)
			p = slash + 1;
		else
			break;
	}

	free(dup);
	*out = cur;
	return 0;
}

// Unlink a directory entry if it exists (for overwrite semantics).
// Also frees the inode if its link count drops to zero.
static errcode_t unlink_if_exists(ext2_filsys fs, ext2_ino_t parent,
				  const char *name)
{
	ext2_ino_t ino;
	errcode_t ret = ext2fs_lookup(fs, parent, name, strlen(name), 0, &ino);
	if (ret == EXT2_ET_FILE_NOT_FOUND)
		return 0; // nothing to remove
	if (ret)
		return ret;

	ret = ext2fs_unlink(fs, parent, name, ino, 0);
	if (ret)
		return ret;

	// Decrement link count; free inode if it reaches zero.
	struct ext2_inode inode;
	ret = ext2fs_read_inode(fs, ino, &inode);
	if (ret)
		return 0; // best effort
	if (inode.i_links_count > 0)
		inode.i_links_count--;
	if (inode.i_links_count == 0) {
		inode.i_dtime = (int32_t)time(NULL);
		ext2fs_inode_alloc_stats2(fs, ino, -1,
					  LINUX_S_ISDIR(inode.i_mode));
	}
	ext2fs_write_inode(fs, ino, &inode);
	return 0;
}

// ---- public API ----

errcode_t e2fs_create(const char *path, uint64_t size_bytes, e2fs_t *out)
{
	errcode_t ret;
	ext2_filsys fs = NULL;
	struct ext2_super_block param;

	memset(&param, 0, sizeof(param));

	unsigned int blocksize = 4096;
	param.s_log_block_size = 2; // log2(4096) - log2(1024) = 2
	param.s_blocks_count = size_bytes / blocksize;

	// Set features BEFORE ext2fs_initialize so it can size structures
	// appropriately (e.g. 64-bit block numbers, metadata checksums).
	{
		__u32 compat_array[3] = {0, 0, 0};
		if (e2p_edit_feature2(MKFS_FEATURES, compat_array, NULL,
				      NULL, NULL, NULL))
			return EXT2_ET_INVALID_ARGUMENT;

		param.s_feature_compat = compat_array[0];
		param.s_feature_incompat = compat_array[1];
		param.s_feature_ro_compat = compat_array[2];
	}

	/* go2fs: EXT2_FLAG_EXCLUSIVE removed — O_EXCL without O_CREAT
	   has undefined behavior in modernc/libc on darwin. Not needed
	   for image-file creation. */
	ret = ext2fs_initialize(path, 0,
				&param, unix_io_manager, &fs);
	if (ret)
		return ret;

	// Mark kbytes_written so the kernel knows this is a modern fs.
	fs->super->s_kbytes_written = 1;

	// go2fs: uuid_generate replaced with getentropy — the full
	// uuid library opens /dev/urandom and calls srand/getpid which
	// panic in modernc/libc on darwin/arm64.
	e2fs_fill_random(fs->super->s_uuid, sizeof(fs->super->s_uuid));

	// Initialize checksum seed from UUID (needed for metadata_csum).
	if (ext2fs_has_feature_csum_seed(fs->super))
		fs->super->s_checksum_seed = ext2fs_crc32c_le(~0,
				fs->super->s_uuid, sizeof(fs->super->s_uuid));
	ext2fs_init_csum_seed(fs);

	// Set checksum type.
	if (ext2fs_has_feature_metadata_csum(fs->super))
		fs->super->s_checksum_type = EXT2_CRC32C_CHKSUM;

	// Directory hash seed and algorithm.
	fs->super->s_def_hash_version = EXT2_HASH_HALF_MD4;
	e2fs_fill_random(fs->super->s_hash_seed, sizeof(fs->super->s_hash_seed));

	// Allocate group tables (inode/block bitmaps and inode tables).
	ret = ext2fs_allocate_tables(fs);
	if (ret) {
		ext2fs_close_free(&fs);
		return ret;
	}

	// Zero inode tables and mark them as zeroed (like write_inode_tables
	// with lazy_itable_init=1, itable_zeroed=1).
	for (dgrp_t i = 0; i < fs->group_desc_count; i++) {
		ext2fs_bg_flags_set(fs, i, EXT2_BG_INODE_ZEROED);
		ext2fs_group_desc_csum_set(fs, i);
	}

	// Write reserved inode checksums (required for metadata_csum).
	if (ext2fs_has_feature_metadata_csum(fs->super)) {
		struct ext2_inode inode;
		memset(&inode, 0, sizeof(inode));
		for (ext2_ino_t ino = 1;
		     ino < EXT2_FIRST_INODE(fs->super); ino++) {
			ext2fs_write_inode(fs, ino, &inode);
		}
	}

	// Create root directory (inode 2).
	ret = ext2fs_mkdir(fs, EXT2_ROOT_INO, EXT2_ROOT_INO, 0);
	if (ret) {
		ext2fs_close_free(&fs);
		return ret;
	}

	// Create lost+found with extra blocks (mke2fs expands it).
	{
		ext2_ino_t lpf;
		fs->umask = 077;
		ret = ext2fs_mkdir(fs, EXT2_ROOT_INO, 0, "lost+found");
		fs->umask = 0;
		if (ret) {
			ext2fs_close_free(&fs);
			return ret;
		}
		ret = ext2fs_lookup(fs, EXT2_ROOT_INO, "lost+found", 10, 0, &lpf);
		if (ret == 0) {
			unsigned int lpf_size = 0;
			for (int i = 1; i < EXT2_NDIR_BLOCKS; i++) {
				if ((lpf_size += fs->blocksize) >= 16*1024 &&
				    lpf_size >= 2 * fs->blocksize)
					break;
				ext2fs_expand_dir(fs, lpf);
			}
		}
	}

	// Reserve inodes below EXT2_FIRST_INODE.
	for (ext2_ino_t ino = EXT2_ROOT_INO + 1;
	     ino < EXT2_FIRST_INODE(fs->super); ino++)
		ext2fs_inode_alloc_stats2(fs, ino, +1, 0);
	ext2fs_mark_ib_dirty(fs);

	// Mark bad-block inode as used.
	ext2fs_mark_inode_bitmap2(fs->inode_map, EXT2_BAD_INO);
	ext2fs_inode_alloc_stats2(fs, EXT2_BAD_INO, +1, 0);
	ext2fs_update_bb_inode(fs, NULL);

	// Create resize inode if the feature is enabled.
	if (ext2fs_has_feature_resize_inode(fs->super)) {
		ret = ext2fs_create_resize_inode(fs);
		if (ret) {
			ext2fs_close_free(&fs);
			return ret;
		}
	}

	// Create journal.
	if (ext2fs_has_feature_journal(fs->super)) {
		struct ext2fs_journal_params jparams;
		memset(&jparams, 0, sizeof(jparams));
		jparams.num_journal_blocks = ext2fs_default_journal_size(
			ext2fs_blocks_count(fs->super));
		if (jparams.num_journal_blocks > 0) {
			/* go2fs: EXT2_MKJOURNAL_NO_MNT_CHECK skips the mount
			   point check which fails for image files in
			   modernc/libc (stat on temp path returns ENOENT). */
			ret = ext2fs_add_journal_inode3(fs, &jparams, ~0ULL,
				EXT2_MKJOURNAL_NO_MNT_CHECK);
			if (ret) {
				ext2fs_close_free(&fs);
				return ret;
			}
		}
	}

	*out = (e2fs_t)fs;
	return 0;
}

errcode_t e2fs_open(const char *path, e2fs_t *out)
{
	errcode_t ret;
	ext2_filsys fs = NULL;

	ret = ext2fs_open(path, EXT2_FLAG_RW | EXT2_FLAG_64BITS,
			  0, 0, unix_io_manager, &fs);
	if (ret)
		return ret;

	ret = ext2fs_read_bitmaps(fs);
	if (ret) {
		ext2fs_close_free(&fs);
		return ret;
	}

	*out = (e2fs_t)fs;
	return 0;
}

errcode_t e2fs_close(e2fs_t handle)
{
	ext2_filsys fs = (ext2_filsys)handle;
	return ext2fs_close_free(&fs);
}

errcode_t e2fs_mkdir(e2fs_t handle, const char *path, uint32_t mode,
		     uint32_t uid, uint32_t gid, int64_t mtime)
{
	ext2_filsys fs = (ext2_filsys)handle;
	errcode_t ret;
	ext2_ino_t parent, ino;

	// Split path into dir + base.
	char *dup1 = strdup(path);
	char *dup2 = strdup(path);
	if (!dup1 || !dup2) {
		free(dup1);
		free(dup2);
		return ENOMEM;
	}
	char *dir = dirname(dup1);
	char *base = basename(dup2);

	ret = find_or_mkdir_path(fs, EXT2_ROOT_INO, dir, &parent);
	if (ret)
		goto out;

	// Check if already exists.
	ret = ext2fs_lookup(fs, parent, base, strlen(base), 0, &ino);
	if (ret == 0) {
		// Already exists, just update metadata.
		goto set_meta;
	}

	ret = ext2fs_mkdir(fs, parent, 0, base);
	if (ret)
		goto out;

	ret = ext2fs_lookup(fs, parent, base, strlen(base), 0, &ino);
	if (ret)
		goto out;

set_meta:
	{
		struct ext2_inode inode;
		ret = ext2fs_read_inode(fs, ino, &inode);
		if (ret)
			goto out;
		inode.i_mode = LINUX_S_IFDIR | (mode & 07777);
		inode.i_uid = uid;
		ext2fs_set_i_uid_high(inode, uid >> 16);
		inode.i_gid = gid;
		ext2fs_set_i_gid_high(inode, gid >> 16);
		ext2fs_inode_xtime_set(&inode, i_atime, mtime);
		ext2fs_inode_xtime_set(&inode, i_ctime, mtime);
		ext2fs_inode_xtime_set(&inode, i_mtime, mtime);
		ret = ext2fs_write_inode(fs, ino, &inode);
	}

out:
	free(dup1);
	free(dup2);
	return ret;
}

errcode_t e2fs_write_file(e2fs_t handle, const char *path, uint32_t mode,
			  uint32_t uid, uint32_t gid, int64_t mtime,
			  const void *data, uint64_t size)
{
	ext2_filsys fs = (ext2_filsys)handle;
	errcode_t ret;
	ext2_ino_t parent, newfile;

	// Split path.
	char *dup1 = strdup(path);
	char *dup2 = strdup(path);
	if (!dup1 || !dup2) {
		free(dup1);
		free(dup2);
		return ENOMEM;
	}
	char *dir = dirname(dup1);
	char *base = basename(dup2);

	ret = find_or_mkdir_path(fs, EXT2_ROOT_INO, dir, &parent);
	if (ret)
		goto out;

	// Remove any existing entry so layers can overwrite files.
	ret = unlink_if_exists(fs, parent, base);
	if (ret)
		goto out;

	// Allocate inode.
	ret = ext2fs_new_inode(fs, parent, 010755, 0, &newfile);
	if (ret)
		goto out;

	ret = ext2fs_link(fs, parent, base, newfile,
			  EXT2_FT_REG_FILE | EXT2FS_LINK_EXPAND);
	if (ret)
		goto out;

	ext2fs_inode_alloc_stats2(fs, newfile, +1, 0);

	{
		struct ext2_inode inode;
		memset(&inode, 0, sizeof(inode));
		inode.i_mode = LINUX_S_IFREG | (mode & 07777);
		inode.i_uid = uid;
		ext2fs_set_i_uid_high(inode, uid >> 16);
		inode.i_gid = gid;
		ext2fs_set_i_gid_high(inode, gid >> 16);
		ext2fs_inode_xtime_set(&inode, i_atime, mtime);
		ext2fs_inode_xtime_set(&inode, i_ctime, mtime);
		ext2fs_inode_xtime_set(&inode, i_mtime, mtime);
		inode.i_links_count = 1;
		ret = ext2fs_inode_size_set(fs, &inode, size);
		if (ret)
			goto out;

		if (ext2fs_has_feature_extents(fs->super)) {
			ext2_extent_handle_t ehandle;
			inode.i_flags &= ~EXT4_EXTENTS_FL;
			ret = ext2fs_extent_open2(fs, newfile, &inode, &ehandle);
			if (ret)
				goto out;
			ext2fs_extent_free(ehandle);
		}

		ret = ext2fs_write_new_inode(fs, newfile, &inode);
		if (ret)
			goto out;
	}

	// Write file data.
	if (size > 0) {
		ext2_file_t e2f;
		ret = ext2fs_file_open(fs, newfile, EXT2_FILE_WRITE, &e2f);
		if (ret)
			goto out;

		const char *ptr = (const char *)data;
		uint64_t remaining = size;
		while (remaining > 0) {
			unsigned int written = 0;
			unsigned int chunk = remaining > (1 << 20) ? (1 << 20) : (unsigned int)remaining;
			ret = ext2fs_file_write(e2f, ptr, chunk, &written);
			if (ret) {
				ext2fs_file_close(e2f);
				goto out;
			}
			if (written == 0) {
				ext2fs_file_close(e2f);
				ret = EIO;
				goto out;
			}
			ptr += written;
			remaining -= written;
		}
		ret = ext2fs_file_close(e2f);
	}

out:
	free(dup1);
	free(dup2);
	return ret;
}

// fix_filetype_proc is a dir_iterate callback that patches the file_type byte
// of a directory entry matching a specific name. Used to work around the ccgo
// operator precedence bug in the transpiled ext2fs_dirent_set_file_type.
static int fix_filetype_proc(ext2_ino_t dir, int entry,
			     struct ext2_dir_entry *dirent, int offset,
			     int blocksize, char *buf, void *priv)
{
	struct fix_filetype_ctx {
		const char *name;
		int namelen;
		int filetype;
	} *ctx = priv;

	int name_len = dirent->name_len & 0xFF;
	if (name_len == ctx->namelen &&
	    memcmp(dirent->name, ctx->name, name_len) == 0) {
		// Patch the file_type byte (high byte of name_len).
		dirent->name_len = (dirent->name_len & 0xFF) |
				   (ctx->filetype << 8);
		return DIRENT_ABORT | DIRENT_CHANGED;
	}
	return 0;
}

errcode_t e2fs_symlink(e2fs_t handle, const char *path, const char *target,
		       uint32_t uid, uint32_t gid, int64_t mtime)
{
	ext2_filsys fs = (ext2_filsys)handle;
	errcode_t ret;
	ext2_ino_t parent, ino;
	unsigned int target_len = strlen(target);

	char *dup1 = strdup(path);
	char *dup2 = strdup(path);
	if (!dup1 || !dup2) {
		free(dup1);
		free(dup2);
		return ENOMEM;
	}
	char *dir = dirname(dup1);
	char *base = basename(dup2);

	ret = find_or_mkdir_path(fs, EXT2_ROOT_INO, dir, &parent);
	if (ret)
		goto out;

	// Remove any existing entry so layers can overwrite symlinks.
	ret = unlink_if_exists(fs, parent, base);
	if (ret)
		goto out;

	// Manual symlink creation — the transpiled ext2fs_symlink has a
	// bug where it creates directory inodes instead of symlink inodes.

	// Allocate inode.
	ret = ext2fs_new_inode(fs, parent, LINUX_S_IFLNK | 0755, 0, &ino);
	if (ret)
		goto out;

	// Build the inode.
	{
		struct ext2_inode inode;
		memset(&inode, 0, sizeof(inode));
		inode.i_mode = LINUX_S_IFLNK | 0777;
		inode.i_uid = uid;
		ext2fs_set_i_uid_high(inode, uid >> 16);
		inode.i_gid = gid;
		ext2fs_set_i_gid_high(inode, gid >> 16);
		inode.i_links_count = 1;
		ext2fs_inode_size_set(fs, &inode, target_len);
		ext2fs_inode_xtime_set(&inode, i_atime, mtime);
		ext2fs_inode_xtime_set(&inode, i_ctime, mtime);
		ext2fs_inode_xtime_set(&inode, i_mtime, mtime);

		// Fast symlink: store target in i_block if it fits.
		if (target_len < sizeof(inode.i_block)) {
			memcpy((char *)&inode.i_block, target, target_len + 1);
		} else {
			// Slow symlink: allocate a data block.
			blk64_t blk;
			char *block_buf;
			ret = ext2fs_get_mem(fs->blocksize, &block_buf);
			if (ret)
				goto out;
			memset(block_buf, 0, fs->blocksize);
			strncpy(block_buf, target, fs->blocksize);

			ret = ext2fs_new_block2(fs, 0, NULL, &blk);
			if (ret) {
				ext2fs_free_mem(&block_buf);
				goto out;
			}

			ext2fs_iblk_set(fs, &inode, 1);
			if (ext2fs_has_feature_extents(fs->super))
				inode.i_flags |= EXT4_EXTENTS_FL;

			ret = ext2fs_write_new_inode(fs, ino, &inode);
			if (ret) {
				ext2fs_free_mem(&block_buf);
				goto out;
			}

			ret = ext2fs_bmap2(fs, ino, &inode, NULL, BMAP_SET,
					   0, NULL, &blk);
			if (ret) {
				ext2fs_free_mem(&block_buf);
				goto out;
			}

			ret = io_channel_write_blk64(fs->io, blk, 1, block_buf);
			ext2fs_free_mem(&block_buf);
			if (ret)
				goto out;

			ext2fs_block_alloc_stats2(fs, blk, +1);
			goto link;
		}

		ret = ext2fs_write_new_inode(fs, ino, &inode);
		if (ret)
			goto out;
	}

link:
	ext2fs_inode_alloc_stats2(fs, ino, +1, 0);

	ret = ext2fs_link(fs, parent, base, ino,
			  EXT2_FT_SYMLINK | EXT2FS_LINK_EXPAND);
	if (ret)
		goto out;

	// Workaround: the transpiled ext2fs_link has an operator precedence
	// bug in the inlined ext2fs_dirent_set_file_type — it computes
	// flags & (mask << 8) instead of (flags & mask) << 8, so the file
	// type is always 0 (EXT2_FT_UNKNOWN). Fix by patching the directory
	// entry's file_type byte via dir_iterate.
	{
		struct fix_filetype_ctx {
			const char *name;
			int namelen;
			int filetype;
		} fctx;
		fctx.name = base;
		fctx.namelen = strlen(base);
		fctx.filetype = EXT2_FT_SYMLINK;
		ext2fs_dir_iterate2(fs, parent, 0, 0,
				    fix_filetype_proc, &fctx);
	}

out:
	free(dup1);
	free(dup2);
	return ret;
}

errcode_t e2fs_mknod(e2fs_t handle, const char *path, uint32_t mode,
		     uint32_t uid, uint32_t gid, int64_t mtime,
		     uint32_t major, uint32_t minor)
{
	ext2_filsys fs = (ext2_filsys)handle;
	errcode_t ret;
	ext2_ino_t parent;

	char *dup1 = strdup(path);
	char *dup2 = strdup(path);
	if (!dup1 || !dup2) {
		free(dup1);
		free(dup2);
		return ENOMEM;
	}
	char *dir = dirname(dup1);
	char *base = basename(dup2);

	ret = find_or_mkdir_path(fs, EXT2_ROOT_INO, dir, &parent);
	if (ret)
		goto out;

	// Remove any existing entry so layers can overwrite nodes.
	ret = unlink_if_exists(fs, parent, base);
	if (ret)
		goto out;

	{
		// Map mode to Linux S_IF* and ext2 filetype.
		unsigned long linux_mode;
		int filetype;
		switch (mode & 0170000) {
		case 0020000: linux_mode = LINUX_S_IFCHR; filetype = EXT2_FT_CHRDEV; break;
		case 0060000: linux_mode = LINUX_S_IFBLK; filetype = EXT2_FT_BLKDEV; break;
		case 0010000: linux_mode = LINUX_S_IFIFO; filetype = EXT2_FT_FIFO; break;
		case 0140000: linux_mode = LINUX_S_IFSOCK; filetype = EXT2_FT_SOCK; break;
		default:
			ret = EXT2_ET_INVALID_ARGUMENT;
			goto out;
		}

		ext2_ino_t ino;
		ret = ext2fs_new_inode(fs, parent, 010755, 0, &ino);
		if (ret)
			goto out;

		ret = ext2fs_link(fs, parent, base, ino,
				  filetype | EXT2FS_LINK_EXPAND);
		if (ret)
			goto out;

		ext2fs_inode_alloc_stats2(fs, ino, +1, 0);

		struct ext2_inode inode;
		memset(&inode, 0, sizeof(inode));
		inode.i_mode = linux_mode | (mode & 07777);
		inode.i_uid = uid;
		ext2fs_set_i_uid_high(inode, uid >> 16);
		inode.i_gid = gid;
		ext2fs_set_i_gid_high(inode, gid >> 16);
		ext2fs_inode_xtime_set(&inode, i_atime, mtime);
		ext2fs_inode_xtime_set(&inode, i_ctime, mtime);
		ext2fs_inode_xtime_set(&inode, i_mtime, mtime);
		inode.i_links_count = 1;

		// Set device numbers for char/block devices.
		if (filetype == EXT2_FT_CHRDEV || filetype == EXT2_FT_BLKDEV) {
			if ((major < 256) && (minor < 256)) {
				inode.i_block[0] = major * 256 + minor;
				inode.i_block[1] = 0;
			} else {
				inode.i_block[0] = 0;
				inode.i_block[1] = (minor & 0xff) | (major << 8) |
						   ((minor & ~0xffU) << 12);
			}
		}

		ret = ext2fs_write_new_inode(fs, ino, &inode);
	}

out:
	free(dup1);
	free(dup2);
	return ret;
}

errcode_t e2fs_hardlink(e2fs_t handle, const char *path, const char *target)
{
	ext2_filsys fs = (ext2_filsys)handle;
	errcode_t ret;
	ext2_ino_t parent, target_ino;

	char *dup1 = strdup(path);
	char *dup2 = strdup(path);
	if (!dup1 || !dup2) {
		free(dup1);
		free(dup2);
		return ENOMEM;
	}
	char *dir = dirname(dup1);
	char *base = basename(dup2);

	// Find the target inode.
	ret = find_or_mkdir_path(fs, EXT2_ROOT_INO, target, &target_ino);
	if (ret)
		goto out;

	// Actually we need to resolve the target as a file, not a dir path.
	// Use ext2fs_namei for full path resolution.
	ret = ext2fs_namei(fs, EXT2_ROOT_INO, EXT2_ROOT_INO, target, &target_ino);
	if (ret)
		goto out;

	ret = find_or_mkdir_path(fs, EXT2_ROOT_INO, dir, &parent);
	if (ret)
		goto out;

	// Remove any existing entry so layers can overwrite hardlinks.
	ret = unlink_if_exists(fs, parent, base);
	if (ret)
		goto out;

	ret = ext2fs_link(fs, parent, base, target_ino,
			  EXT2_FT_UNKNOWN | EXT2FS_LINK_EXPAND);
	if (ret)
		goto out;

	// Bump link count.
	{
		struct ext2_inode inode;
		ret = ext2fs_read_inode(fs, target_ino, &inode);
		if (ret)
			goto out;
		inode.i_links_count++;
		ret = ext2fs_write_inode(fs, target_ino, &inode);
	}

out:
	free(dup1);
	free(dup2);
	return ret;
}
