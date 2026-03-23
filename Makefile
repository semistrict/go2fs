E2FS := third_party/e2fsprogs
OUTDIR := internal/e2fs
CCGO := ccgo
CCGO_FLAGS := -c -ignore-unsupported-alignment -DHAVE_CONFIG_H -D'__attribute__(x)=' \
	-I$(E2FS)/lib \
	-I$(E2FS)/lib/ext2fs \
	-I$(E2FS)/lib/blkid \
	-I$(E2FS)/lib/e2p \
	-I$(E2FS)/lib/et \
	-I$(E2FS)/lib/support \
	-I$(E2FS)/lib/uuid \
	-I$(E2FS)/include \
	-I$(E2FS)/misc \
	-I$(E2FS)

# ext2fs library sources
EXT2FS_SRCS := \
	$(E2FS)/lib/ext2fs/ext2_err.c \
	$(E2FS)/lib/ext2fs/alloc.c \
	$(E2FS)/lib/ext2fs/alloc_sb.c \
	$(E2FS)/lib/ext2fs/alloc_stats.c \
	$(E2FS)/lib/ext2fs/alloc_tables.c \
	$(E2FS)/lib/ext2fs/atexit.c \
	$(E2FS)/lib/ext2fs/badblocks.c \
	$(E2FS)/lib/ext2fs/bb_inode.c \
	$(E2FS)/lib/ext2fs/bitmaps.c \
	$(E2FS)/lib/ext2fs/bitops.c \
	$(E2FS)/lib/ext2fs/blkmap64_ba.c \
	$(E2FS)/lib/ext2fs/blkmap64_rb.c \
	$(E2FS)/lib/ext2fs/blknum.c \
	$(E2FS)/lib/ext2fs/block.c \
	$(E2FS)/lib/ext2fs/bmap.c \
	$(E2FS)/lib/ext2fs/check_desc.c \
	$(E2FS)/lib/ext2fs/closefs.c \
	$(E2FS)/lib/ext2fs/crc16.c \
	$(E2FS)/lib/ext2fs/crc32c.c \
	$(E2FS)/lib/ext2fs/csum.c \
	$(E2FS)/lib/ext2fs/dblist.c \
	$(E2FS)/lib/ext2fs/dblist_dir.c \
	$(E2FS)/lib/ext2fs/dirblock.c \
	$(E2FS)/lib/ext2fs/dirhash.c \
	$(E2FS)/lib/ext2fs/dir_iterate.c \
	$(E2FS)/lib/ext2fs/expanddir.c \
	$(E2FS)/lib/ext2fs/ext_attr.c \
	$(E2FS)/lib/ext2fs/extent.c \
	$(E2FS)/lib/ext2fs/fallocate.c \
	$(E2FS)/lib/ext2fs/fileio.c \
	$(E2FS)/lib/ext2fs/finddev.c \
	$(E2FS)/lib/ext2fs/flushb.c \
	$(E2FS)/lib/ext2fs/freefs.c \
	$(E2FS)/lib/ext2fs/gen_bitmap.c \
	$(E2FS)/lib/ext2fs/gen_bitmap64.c \
	$(E2FS)/lib/ext2fs/get_num_dirs.c \
	$(E2FS)/lib/ext2fs/get_pathname.c \
	$(E2FS)/lib/ext2fs/getenv.c \
	$(E2FS)/lib/ext2fs/getsize.c \
	$(E2FS)/lib/ext2fs/getsectsize.c \
	$(E2FS)/lib/ext2fs/hashmap.c \
	$(E2FS)/lib/ext2fs/i_block.c \
	$(E2FS)/lib/ext2fs/icount.c \
	$(E2FS)/lib/ext2fs/ind_block.c \
	$(E2FS)/lib/ext2fs/initialize.c \
	$(E2FS)/lib/ext2fs/inline.c \
	$(E2FS)/lib/ext2fs/inline_data.c \
	$(E2FS)/lib/ext2fs/inode.c \
	$(E2FS)/lib/ext2fs/io_manager.c \
	$(E2FS)/lib/ext2fs/ismounted.c \
	$(E2FS)/lib/ext2fs/link.c \
	$(E2FS)/lib/ext2fs/llseek.c \
	$(E2FS)/lib/ext2fs/lookup.c \
	$(E2FS)/lib/ext2fs/mkdir.c \
	$(E2FS)/lib/ext2fs/mkjournal.c \
	$(E2FS)/lib/ext2fs/mmp.c \
	$(E2FS)/lib/ext2fs/namei.c \
	$(E2FS)/lib/ext2fs/native.c \
	$(E2FS)/lib/ext2fs/newdir.c \
	$(E2FS)/lib/ext2fs/nls_utf8.c \
	$(E2FS)/lib/ext2fs/openfs.c \
	$(E2FS)/lib/ext2fs/orphan.c \
	$(E2FS)/lib/ext2fs/progress.c \
	$(E2FS)/lib/ext2fs/punch.c \
	$(E2FS)/lib/ext2fs/qcow2.c \
	$(E2FS)/lib/ext2fs/read_bb.c \
	$(E2FS)/lib/ext2fs/read_bb_file.c \
	$(E2FS)/lib/ext2fs/res_gdt.c \
	$(E2FS)/lib/ext2fs/rw_bitmaps.c \
	$(E2FS)/lib/ext2fs/sha512.c \
	$(E2FS)/lib/ext2fs/swapfs.c \
	$(E2FS)/lib/ext2fs/symlink.c \
	$(E2FS)/lib/ext2fs/tdb.c \
	$(E2FS)/lib/ext2fs/undo_io.c \
	$(E2FS)/lib/ext2fs/unix_io.c \
	$(E2FS)/lib/ext2fs/sparse_io.c \
	$(E2FS)/lib/ext2fs/unlink.c \
	$(E2FS)/lib/ext2fs/valid_blk.c \
	$(E2FS)/lib/ext2fs/version.c \
	$(E2FS)/lib/ext2fs/rbtree.c \
	$(E2FS)/lib/ext2fs/test_io.c

# com_err (et) library
ET_SRCS := \
	$(E2FS)/lib/et/error_message.c \
	$(E2FS)/lib/et/et_name.c \
	$(E2FS)/lib/et/init_et.c \
	$(E2FS)/lib/et/com_err.c \
	$(E2FS)/lib/et/com_right.c

# uuid library
UUID_SRCS := \
	$(E2FS)/lib/uuid/clear.c \
	$(E2FS)/lib/uuid/compare.c \
	$(E2FS)/lib/uuid/copy.c \
	$(E2FS)/lib/uuid/gen_uuid.c \
	$(E2FS)/lib/uuid/isnull.c \
	$(E2FS)/lib/uuid/pack.c \
	$(E2FS)/lib/uuid/parse.c \
	$(E2FS)/lib/uuid/unpack.c \
	$(E2FS)/lib/uuid/unparse.c \
	$(E2FS)/lib/uuid/uuid_time.c

# e2p library
E2P_SRCS := \
	$(E2FS)/lib/e2p/feature.c \
	$(E2FS)/lib/e2p/hashstr.c \
	$(E2FS)/lib/e2p/iod.c \
	$(E2FS)/lib/e2p/ls.c \
	$(E2FS)/lib/e2p/mntopts.c \
	$(E2FS)/lib/e2p/ostype.c \
	$(E2FS)/lib/e2p/parse_num.c \
	$(E2FS)/lib/e2p/pe.c \
	$(E2FS)/lib/e2p/crypto_mode.c \
	$(E2FS)/lib/e2p/encoding.c \
	$(E2FS)/lib/e2p/errcode.c \
	$(E2FS)/lib/e2p/fgetflags.c \
	$(E2FS)/lib/e2p/fsetflags.c \
	$(E2FS)/lib/e2p/fgetversion.c \
	$(E2FS)/lib/e2p/fsetversion.c \
	$(E2FS)/lib/e2p/getflags.c \
	$(E2FS)/lib/e2p/getversion.c \
	$(E2FS)/lib/e2p/fgetproject.c \
	$(E2FS)/lib/e2p/fsetproject.c \
	$(E2FS)/lib/e2p/ljs.c \
	$(E2FS)/lib/e2p/uuid.c \
	$(E2FS)/lib/e2p/ps.c

# blkid library
BLKID_SRCS := \
	$(E2FS)/lib/blkid/cache.c \
	$(E2FS)/lib/blkid/dev.c \
	$(E2FS)/lib/blkid/devname.c \
	$(E2FS)/lib/blkid/devno.c \
	$(E2FS)/lib/blkid/getsize.c \
	$(E2FS)/lib/blkid/llseek.c \
	$(E2FS)/lib/blkid/probe.c \
	$(E2FS)/lib/blkid/read.c \
	$(E2FS)/lib/blkid/resolve.c \
	$(E2FS)/lib/blkid/save.c \
	$(E2FS)/lib/blkid/tag.c

# support library (quota files excluded - use GNU variadic macros unsupported by ccgo)
SUPPORT_SRCS := \
	$(E2FS)/lib/support/argv_parse.c \
	$(E2FS)/lib/support/cstring.c \
	$(E2FS)/lib/support/dict.c \
	$(E2FS)/lib/support/plausible.c \
	$(E2FS)/lib/support/print_fs_flags.c \
	$(E2FS)/lib/support/prof_err.c \
	$(E2FS)/lib/support/profile_helpers.c \
	$(E2FS)/lib/support/profile.c

# High-level C wrapper (e2fs_create, e2fs_mkdir, etc.)
# Prefixed with _ so Go's build system ignores it.
IMPL_SRCS := _e2fs_impl.c

# We build only the libraries - the Go wrapper will call libext2fs functions
# directly to create filesystems and populate them from tarballs (read via Go stdlib).
ALL_SRCS := $(EXT2FS_SRCS) $(ET_SRCS) $(UUID_SRCS) $(E2P_SRCS) $(BLKID_SRCS) $(SUPPORT_SRCS) $(IMPL_SRCS)

# Generated .o.go files go into OUTDIR
E2FS_GO_OBJS := $(patsubst $(E2FS)/%.c,$(OUTDIR)/%.o.go,$(filter $(E2FS)/%,$(ALL_SRCS)))
GO_OBJS := $(E2FS_GO_OBJS) $(OUTDIR)/e2fs_impl.o.go

.PHONY: all clean configure generate test

all: generate

test:
	go test -race -gcflags=all=-d=checkptr=0 -count=1 ./...

# Run configure to generate config.h, blkid.h, ext2_err.c/h, prof_err.c/h
configure: $(E2FS)/lib/config.h

$(E2FS)/lib/config.h:
	cd $(E2FS) && ./configure --disable-nls --disable-defrag --disable-fuse2fs \
		--disable-debugfs --disable-imager --disable-resizer \
		--disable-e2initrd-helper --disable-uuidd --disable-fsck --disable-tls

# Generate error tables and blkid.h
$(E2FS)/lib/ext2fs/ext2_err.c $(E2FS)/lib/ext2fs/ext2_err.h: $(E2FS)/lib/config.h
	$(MAKE) -C $(E2FS)/lib/et
	$(MAKE) -C $(E2FS)/lib/ext2fs ext2_err.c ext2_err.h

$(E2FS)/lib/ext2fs/crc32c_table.h: $(E2FS)/lib/config.h
	$(MAKE) -C $(E2FS)/lib/ext2fs crc32c_table.h

$(E2FS)/lib/support/prof_err.c $(E2FS)/lib/support/prof_err.h: $(E2FS)/lib/config.h
	$(MAKE) -C $(E2FS)/lib/support prof_err.c prof_err.h

$(E2FS)/lib/blkid/blkid.h: $(E2FS)/lib/config.h
	$(MAKE) -C $(E2FS)/lib/blkid blkid.h

$(E2FS)/lib/uuid/uuid.h: $(E2FS)/lib/config.h
	$(MAKE) -C $(E2FS)/lib/uuid uuid.h

# Pattern rule: compile each C file to a .o.go file in OUTDIR
$(OUTDIR)/%.o.go: $(E2FS)/%.c $(E2FS)/lib/config.h $(E2FS)/lib/ext2fs/ext2_err.h $(E2FS)/lib/support/prof_err.h $(E2FS)/lib/blkid/blkid.h $(E2FS)/lib/uuid/uuid.h $(E2FS)/lib/ext2fs/crc32c_table.h
	@mkdir -p $(dir $@)
	$(CCGO) $(CCGO_FLAGS) -o $@ $<

# Compile files not under E2FS
$(OUTDIR)/e2fs_impl.o.go: _e2fs_impl.c _e2fs_impl.h
	@mkdir -p $(dir $@)
	$(CCGO) $(CCGO_FLAGS) -I. -o $@ $<

# Link all .o.go files into the final Go package
generate: $(GO_OBJS)
	$(CCGO) -ignore-link-errors --package-name e2fs -o $(OUTDIR)/e2fs.go $(GO_OBJS)

# On a linux/amd64 host, `make generate` produces e2fs.go with
# //go:build linux && amd64. Use `make generate-linux-amd64` on
# a linux runner to regenerate for that platform.
generate-linux-amd64:
	$(MAKE) clean
	$(MAKE) generate

clean:
	find $(OUTDIR) -name '*.o.go' -delete 2>/dev/null; rm -f $(OUTDIR)/e2fs.go
	find $(OUTDIR) -type d -empty -delete 2>/dev/null; true
