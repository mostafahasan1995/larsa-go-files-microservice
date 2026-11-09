#first stage - builder
FROM golang:1.19-bullseye as builder

# Installs libvips + required libraries for optimization
ARG LIBVIPS_VERSION=8.10.0

RUN apt-get update -y
RUN apt-get install git zlib1g-dev -y 
RUN git clone --depth 1 https://github.com/gpac/gpac.git gpac_public && \
    cd gpac_public && \
    ./configure --static-mp4box && \
    make && \
    make install

# Installs libvips + required libraries
RUN DEBIAN_FRONTEND=noninteractive \
  apt-get install --no-install-recommends -y \
  ca-certificates \
  automake build-essential curl \
  gobject-introspection gtk-doc-tools libglib2.0-dev libjpeg62-turbo-dev libpng-dev \
  libwebp-dev libtiff5-dev libgif-dev libexif-dev libxml2-dev libpoppler-glib-dev \
  swig libmagickwand-dev libpango1.0-dev libmatio-dev libopenslide-dev libcfitsio-dev \
  libgsf-1-dev fftw3-dev liborc-0.4-dev librsvg2-dev libimagequant-dev libopenexr25 libheif-dev libvips-dev
  # cd /tmp && \
  # curl -fsSLO https://github.com/libvips/libvips/releases/download/v${LIBVIPS_VERSION}/vips-${LIBVIPS_VERSION}.tar.gz && \
  # tar zvxf vips-${LIBVIPS_VERSION}.tar.gz && \
  # cd /tmp/vips-${LIBVIPS_VERSION} && \
  # CFLAGS="-g -O3" CXXFLAGS="-D_GLIBCXX_USE_CXX11_ABI=0 -g -O3" \
  # ./configure \
  # --disable-debug \
  # --disable-dependency-tracking \
  # --disable-introspection \
  # --disable-static \
  # --enable-gtk-doc-html=no \
  # --enable-gtk-doc=no \
  # --enable-pyvips8=no && \
  # make && \
  # make install && \
  # ldconfig

COPY . /app
WORKDIR /app

ARG GITLAB_ACCESS_TOKEN
RUN git config --global url."https://oauth2:${GITLAB_ACCESS_TOKEN}@git.larsa.io/".insteadOf "https://git.larsa.io/"

ENV GO111MODULE=on
RUN CGO_ENABLED=1 GOOS=linux go build -o main

#second stage
FROM debian:bookworm-slim

WORKDIR /app

COPY --from=builder /usr/local/lib /usr/local/lib
COPY --from=builder /usr/local/bin/MP4Box /usr/local/bin/MP4Box
COPY --from=builder /app/main .
COPY --from=builder /etc/ssl/certs /etc/ssl/certs

# Install runtime dependencies
RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y \
    ca-certificates \
    automake build-essential curl \
    gobject-introspection gtk-doc-tools libglib2.0-dev libjpeg62-turbo-dev libpng-dev \
    libwebp-dev libtiff5-dev libgif-dev libexif-dev libxml2-dev libpoppler-glib-dev \
    swig libmagickwand-dev libpango1.0-dev libmatio-dev libopenslide-dev libcfitsio-dev \
    libgsf-1-dev libfftw3-dev liborc-0.4-dev librsvg2-dev libimagequant-dev libopenexr-dev libheif-dev libvips-dev  && \
  apt-get autoremove -y && \
  apt-get autoclean && \
  apt-get clean && \
  rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

CMD ["./main"]

# #first stage - builder
# FROM golang:1.18 as builder

# # Installs libvips + required libraries for optimization
# ARG LIBVIPS_VERSION=8.10.0

# RUN apt-get update -y
# RUN apt-get install git zlib1g-dev -y 
# RUN git clone https://github.com/gpac/gpac.git gpac_public && \
#   cd gpac_public \
#   ./configure --static-mp4box && \
#   make && \
#   make install 

# # Installs libvips + required libraries
# RUN DEBIAN_FRONTEND=noninteractive \
#   apt-get install --no-install-recommends -y \
#   ffmpeg \
#   ca-certificates \
#   automake build-essential curl \
#   gobject-introspection gtk-doc-tools libglib2.0-dev libjpeg62-turbo-dev libpng-dev \
#   libwebp-dev libtiff5-dev libgif-dev libexif-dev libxml2-dev libpoppler-glib-dev \
#   swig libmagickwand-dev libpango1.0-dev libmatio-dev libopenslide-dev libcfitsio-dev \
#   libgsf-1-dev fftw3-dev liborc-0.4-dev librsvg2-dev libimagequant-dev libopenexr25 libheif-dev libvips-dev
#   # cd /tmp && \
#   # curl -fsSLO https://github.com/libvips/libvips/releases/download/v${LIBVIPS_VERSION}/vips-${LIBVIPS_VERSION}.tar.gz && \
#   # tar zvxf vips-${LIBVIPS_VERSION}.tar.gz && \
#   # cd /tmp/vips-${LIBVIPS_VERSION} && \
#   # CFLAGS="-g -O3" CXXFLAGS="-D_GLIBCXX_USE_CXX11_ABI=0 -g -O3" \
#   # ./configure \
#   # --disable-debug \
#   # --disable-dependency-tracking \
#   # --disable-introspection \
#   # --disable-static \
#   # --enable-gtk-doc-html=no \
#   # --enable-gtk-doc=no \
#   # --enable-pyvips8=no && \
#   # make && \
#   # make install && \
#   # ldconfig

# COPY . /app
# WORKDIR /app

# ARG GITLAB_ACCESS_TOKEN
# RUN git config --global url."https://oauth2:${GITLAB_ACCESS_TOKEN}@git.larsa.io/".insteadOf "https://git.larsa.io/"

# ENV GO111MODULE=on
# RUN CGO_ENABLED=1 GOOS=linux go build -o main

# #second stage
# FROM debian:buster-slim

# WORKDIR /app

# COPY --from=builder /usr/local/lib /usr/local/lib
# COPY --from=builder /usr/local/bin/MP4Box /usr/local/bin/MP4Box
# COPY --from=builder /app/main .
# COPY --from=builder /etc/ssl/certs /etc/ssl/certs

# # Install runtime dependencies
# RUN DEBIAN_FRONTEND=noninteractive \
#   apt-get update && \
#   apt-get install ffmpeg -y && \
#   apt-get install --no-install-recommends -y \
#   procps libglib2.0-0 libjpeg62-turbo libpng16-16 libopenexr23 \
#   libwebp6 libwebpmux3 libwebpdemux2 libtiff5 libgif7 libexif12 libxml2 libpoppler-glib8 \
#   libmagickwand-6.q16-6 libpango1.0-0 libmatio4 libopenslide0 \
#   libgsf-1-114 fftw3 liborc-0.4-0 librsvg2-2 libcfitsio7 libimagequant0 libheif1 libvips-dev && \
#   apt-get autoremove -y && \
#   apt-get autoclean && \
#   apt-get clean && \
#   rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# CMD ["./main"]