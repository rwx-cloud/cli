package accesstoken_test

import (
	"io"
	"io/fs"
	"os"
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rwx-research/mint-cli/internal/accesstoken"
)

var _ = Describe("FileBackend", func() {
	var primaryTmpDir string
	var fallbackTmpDir string

	BeforeEach(func() {
		var err error
		primaryTmpDir, err = os.MkdirTemp(os.TempDir(), "file-backend-primary")
		Expect(err).NotTo(HaveOccurred())

		fallbackTmpDir, err = os.MkdirTemp(os.TempDir(), "file-backend-fallback")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		var err error

		err = os.RemoveAll(primaryTmpDir)
		Expect(err).NotTo(HaveOccurred())

		err = os.RemoveAll(fallbackTmpDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Get", func() {
		Describe("when there is only a single directory", func() {
			Context("when the access token file does not exist", func() {
				It("returns an empty token", func() {
					backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir})
					Expect(err).NotTo(HaveOccurred())

					token, err := backend.Get()
					Expect(err).NotTo(HaveOccurred())
					Expect(token).To(Equal(""))
				})
			})

			Context("when the access token file is otherwise unable to be opened", func() {
				BeforeEach(func() {
					Expect(os.Chmod(primaryTmpDir, 0o000)).NotTo(HaveOccurred())
				})

				It("returns an error", func() {
					backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir})
					Expect(err).NotTo(HaveOccurred())

					token, err := backend.Get()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("unable to open"))
					Expect(err).To(MatchError(fs.ErrPermission))
					Expect(token).To(Equal(""))
				})
			})

			Context("when the access token file is present and has contents", func() {
				BeforeEach(func() {
					err := os.WriteFile(path.Join(primaryTmpDir, "accesstoken"), []byte("the-token"), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the token", func() {
					backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir})
					Expect(err).NotTo(HaveOccurred())

					token, err := backend.Get()
					Expect(err).NotTo(HaveOccurred())
					Expect(token).To(Equal("the-token"))
				})
			})

			Context("when the access token file includes leading or trailing whitespace", func() {
				BeforeEach(func() {
					err := os.WriteFile(path.Join(primaryTmpDir, "accesstoken"), []byte("\n  \t  the-token\t  \n \n"), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the token with surrounding whitespace trimmed", func() {
					backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir})
					Expect(err).NotTo(HaveOccurred())

					token, err := backend.Get()
					Expect(err).NotTo(HaveOccurred())
					Expect(token).To(Equal("the-token"))
				})
			})
		})

		Describe("when there are multiple directories", func() {
			Context("when the access token file does not exist in either directory", func() {
				It("returns an empty token", func() {
					backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
					Expect(err).NotTo(HaveOccurred())

					token, err := backend.Get()
					Expect(err).NotTo(HaveOccurred())
					Expect(token).To(Equal(""))
				})
			})

			Context("when the access token file exists in the primary but not the fallback", func() {
				BeforeEach(func() {
					err := os.WriteFile(path.Join(primaryTmpDir, "accesstoken"), []byte("the-token"), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the token and does not write one to the fallback", func() {
					backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
					Expect(err).NotTo(HaveOccurred())

					token, err := backend.Get()
					Expect(err).NotTo(HaveOccurred())
					Expect(token).To(Equal("the-token"))

					_, err = os.Stat(path.Join(fallbackTmpDir, "accesstoken"))
					Expect(os.IsNotExist(err)).To(BeTrue())
				})
			})

			Context("when the access token file exists in the fallback but not the primary", func() {
				BeforeEach(func() {
					err := os.WriteFile(path.Join(fallbackTmpDir, "accesstoken"), []byte("the-token"), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the token and writes it to the primary", func() {
					backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
					Expect(err).NotTo(HaveOccurred())

					token, err := backend.Get()
					Expect(err).NotTo(HaveOccurred())
					Expect(token).To(Equal("the-token"))

					file, err := os.Open(path.Join(primaryTmpDir, "accesstoken"))
					Expect(err).NotTo(HaveOccurred())
					bytes, err := io.ReadAll(file)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(bytes)).To(Equal("the-token"))
				})
			})

			Context("when the access token file in the primary dir is otherwise unable to be opened", func() {
				BeforeEach(func() {
					Expect(os.Chmod(primaryTmpDir, 0o000)).NotTo(HaveOccurred())
				})

				It("returns an error", func() {
					backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
					Expect(err).NotTo(HaveOccurred())

					token, err := backend.Get()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("unable to open"))
					Expect(err).To(MatchError(fs.ErrPermission))
					Expect(token).To(Equal(""))
				})
			})

			Context("when the access token file in the primary dir includes leading or trailing whitespace", func() {
				BeforeEach(func() {
					err := os.WriteFile(path.Join(primaryTmpDir, "accesstoken"), []byte("\n  \t  the-token\t  \n \n"), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the token with surrounding whitespace trimmed", func() {
					backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
					Expect(err).NotTo(HaveOccurred())

					token, err := backend.Get()
					Expect(err).NotTo(HaveOccurred())
					Expect(token).To(Equal("the-token"))
				})
			})

			Context("when the access token file in the fallback dir is otherwise unable to be opened", func() {
				BeforeEach(func() {
					Expect(os.Chmod(fallbackTmpDir, 0o000)).NotTo(HaveOccurred())
				})

				It("returns an error", func() {
					backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
					Expect(err).NotTo(HaveOccurred())

					token, err := backend.Get()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("unable to open"))
					Expect(err).To(MatchError(fs.ErrPermission))
					Expect(token).To(Equal(""))
				})
			})

			Context("when the access token file in the fallback dir includes leading or trailing whitespace", func() {
				BeforeEach(func() {
					err := os.WriteFile(path.Join(fallbackTmpDir, "accesstoken"), []byte("\n  \t  the-token\t  \n \n"), 0o644)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the token with surrounding whitespace trimmed", func() {
					backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir, fallbackTmpDir})
					Expect(err).NotTo(HaveOccurred())

					token, err := backend.Get()
					Expect(err).NotTo(HaveOccurred())
					Expect(token).To(Equal("the-token"))
				})
			})
		})
	})

	Describe("Set", func() {
		Context("when creating the file errors", func() {
			BeforeEach(func() {
				Expect(os.Chmod(primaryTmpDir, 0o400)).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir})
				Expect(err).NotTo(HaveOccurred())

				err = backend.Set("the-token")
				Expect(err.Error()).To(ContainSubstring("permission denied"))
				Expect(err).To(MatchError(fs.ErrPermission))
			})
		})

		Context("when the file is created", func() {
			It("writes the token to the file", func() {
				backend, err := accesstoken.NewFileBackend([]string{primaryTmpDir})
				Expect(err).NotTo(HaveOccurred())

				err = backend.Set("the-token")
				Expect(err).NotTo(HaveOccurred())

				file, err := os.Open(path.Join(primaryTmpDir, "accesstoken"))
				Expect(err).NotTo(HaveOccurred())

				bytes, err := io.ReadAll(file)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(bytes)).To(Equal("the-token"))
			})
		})
	})
})
