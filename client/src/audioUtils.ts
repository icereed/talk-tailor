import { createFFmpeg, fetchFile } from "@ffmpeg/ffmpeg";

export async function convertWAVtoMP3(blob: Blob): Promise<Blob> {
  const reader = new FileReader();
  const arrayBuffer: ArrayBuffer = await new Promise((resolve) => {
    reader.onload = (event) => resolve(event.target.result as ArrayBuffer);
    reader.readAsArrayBuffer(blob);
  });

  const audioContext = new AudioContext();
  const buffer = await audioContext.decodeAudioData(arrayBuffer);

  // Resample the audio buffer to 44100 Hz
  const targetSampleRate = 44100;
  const resampledBuffer = await resampleAudioBuffer(buffer, targetSampleRate);

  const wavData = convertFloat32ToInt16(resampledBuffer.getChannelData(0));
  const mp3Data = await encodeMP3(wavData, targetSampleRate);
  return new Blob(mp3Data, { type: "audio/mpeg" });
}

export async function sendMP3ToBackend(mp3Blob: Blob): Promise<string> {
  const formData = new FormData();
  formData.append("audio", mp3Blob, "audio.mp3");

  try {
    const host = window.location.host;
    const protcol = window.location.protocol;
    const endpoint = `${protcol}//${host}/api/transcribe`;
    const response = await fetch(endpoint, {
      method: "POST",
      body: formData,
    });

    if (!response.ok) {
      throw new Error("Failed to upload audio");
    }

    const transcription = await response.json();
    return transcription.transcription;
  } catch (error) {
    console.error(error);
    throw new Error("Error uploading and processing audio");
  }
}

export async function resampleAudioBuffer(
  buffer: AudioBuffer,
  targetSampleRate: number
): Promise<AudioBuffer> {
  const numberOfChannels = buffer.numberOfChannels;
  const originalSampleRate = buffer.sampleRate;
  const duration = buffer.duration;

  if (originalSampleRate === targetSampleRate) {
    return buffer;
  }

  const offlineContext = new OfflineAudioContext(
    numberOfChannels,
    duration * targetSampleRate,
    targetSampleRate
  );
  const source = offlineContext.createBufferSource();
  source.buffer = buffer;
  source.connect(offlineContext.destination);
  source.start(0);

  const resampledBuffer = await offlineContext.startRendering();
  return resampledBuffer;
}

export function convertFloat32ToInt16(buffer: Float32Array): Int16Array {
  const length = buffer.length;
  const output = new Int16Array(length);

  for (let i = 0; i < length; i++) {
    output[i] = Math.min(1, buffer[i]) * 0x7fff;
  }

  return output;
}

function createWavHeader(
  sampleRate: number,
  numChannels: number,
  numSamples: number
): DataView {
  const buffer = new ArrayBuffer(44);
  const view = new DataView(buffer);

  const writeString = (view: DataView, offset: number, string: string) => {
    for (let i = 0; i < string.length; i++) {
      view.setUint8(offset + i, string.charCodeAt(i));
    }
  };

  writeString(view, 0, "RIFF");
  view.setUint32(4, 36 + numSamples * 2, true);
  writeString(view, 8, "WAVE");
  writeString(view, 12, "fmt ");
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, numChannels, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, sampleRate * numChannels * 2, true);
  view.setUint16(32, numChannels * 2, true);
  view.setUint16(34, 16, true);
  writeString(view, 36, "data");
  view.setUint32(40, numSamples * 2, true);

  return view;
}

async function encodeMP3(wavData: Int16Array, sampleRate: number): Promise<Int8Array[]> {
  const kbps = 128;
  const channels = 1;

  // Create a WAV Blob from the Int16Array data
  const wavHeader = createWavHeader(sampleRate, channels, wavData.length);
  const wavBuffer = new ArrayBuffer(wavData.length * 2 + 44);
  const view = new DataView(wavBuffer);
  new Int8Array(wavBuffer).set(new Int8Array(wavHeader.buffer), 0);

  for (let i = 0; i < wavData.length; i++) {
    view.setInt16(44 + i * 2, wavData[i], true);
  }

  const wavBlob = new Blob([wavBuffer], { type: "audio/wav" });

  // Initialize FFmpeg
  const ffmpeg = createFFmpeg({ log: false });
  await ffmpeg.load();

  // Convert WAV Blob to MP3
  ffmpeg.FS("writeFile", "input.wav", await fetchFile(wavBlob));
  await ffmpeg.run(
    "-i",               // Input flag
    "input.wav",        // Input WAV file
    "-acodec",          // Audio codec flag
    "libmp3lame",       // Audio codec name (MP3 encoder)
    "-b:a",             // Bitrate flag
    `${kbps}k`,         // Bitrate value (128 kbps)
    "-ac",              // Number of audio channels flag
    channels.toString(),// Number of audio channels (1)
    "-ar",              // Audio sampling frequency flag
    sampleRate.toString(), // Audio sampling frequency (44100 Hz)
    "output.mp3"        // Output MP3 file
  );


  // Get the MP3 data and return it
  const mp3Data = await ffmpeg.FS("readFile", "output.mp3");
  return [new Int8Array(mp3Data)];
}

export async function convertVideoToMP3(videoBlob: Blob): Promise<Blob> {
  // Initialize FFmpeg
  const ffmpeg = createFFmpeg({ log: false });
  await ffmpeg.load();

  // Write the video file to FFmpeg's virtual file system
  ffmpeg.FS("writeFile", "input.video", await fetchFile(videoBlob));

  // Convert the video file to MP3
  await ffmpeg.run(
    "-i",          // Input flag
    "input.video", // Input video file
    "-vn",         // Disable video stream
    "-acodec",     // Audio codec flag
    "libmp3lame",  // Audio codec name (MP3 encoder)
    "output.mp3"   // Output MP3 file
  );

  // Get the MP3 data and return it as a Blob
  const mp3Data = await ffmpeg.FS("readFile", "output.mp3");
  return new Blob([mp3Data.buffer], { type: "audio/mpeg" });
}
