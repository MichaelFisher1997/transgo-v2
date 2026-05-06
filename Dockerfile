FROM jellyfin/jellyfin:10.11.6

USER root
RUN ln -sf /usr/lib/jellyfin-ffmpeg/ffmpeg /usr/bin/ffmpeg \
    && ln -s /usr/lib/jellyfin-ffmpeg/ffprobe /usr/bin/ffprobe
ENV LIBVA_DRIVERS_PATH=/usr/lib/jellyfin-ffmpeg/lib/dri
ENV LIBVA_DRIVER_NAME=iHD
WORKDIR /app
COPY frontend/dist ./frontend/dist
COPY transgo .
EXPOSE 8080
ENTRYPOINT []
CMD ["./transgo"]
