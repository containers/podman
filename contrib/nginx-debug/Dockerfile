FROM nginx:latest
RUN sed -i -r -e 's#error_log\s+\/var\/log\/nginx\/error.log\s+warn\;#error_log  /var/log/nginx/error.log debug;#' /etc/nginx/nginx.conf
CMD ["nginx-debug", "-g", "daemon off;"]
