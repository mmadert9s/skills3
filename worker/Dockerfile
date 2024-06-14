FROM golang:1.22 as gobuild

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN go build -v -o worker ./...

FROM python:3.7-slim

COPY --from=gobuild /usr/src/app/worker /usr/local/bin/worker

COPY ./facerecognition/yolo_opencv.py /workdir/yolo_opencv.py
COPY ./facerecognition/yolov3.cfg /workdir/yolov3.cfg
COPY ./facerecognition/yolov3.txt  /workdir/yolov3.txt 
COPY ./facerecognition/yolov3.weights /workdir/yolov3.weights

RUN apt-get update && apt-get -y install --no-install-recommends \
    python3 python3-numpy python3-setuptools python3-pip \
    libopencv-dev opencv-data
    
RUN pip3 install opencv-python

CMD ["worker"]