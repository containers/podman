FROM alpine
WORKDIR /app
RUN apk update && apk add py3-pip && pip3 install flask
COPY . /app
ENTRYPOINT ["python3"]
CMD ["app.py"]
