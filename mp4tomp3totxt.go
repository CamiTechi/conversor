package main

import (
	"bufio"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Embarcando o FFmpeg no executável
// Você precisará baixar os binários do FFmpeg e colocá-los em uma pasta "ffmpeg"
// Windows: ffmpeg/windows/ffmpeg.exe
// Linux: ffmpeg/linux/ffmpeg
// macOS: ffmpeg/darwin/ffmpeg

//go:embed ffmpeg/*
var embeddedFFmpeg embed.FS

func main() {
	if len(os.Args) == 1 || (len(os.Args) > 1 && os.Args[1] == "gui") {
		runGUI() //chamando função do gui.go
		return
	}
	var (
		typeFile    string
		inputPath   string
		outputPath  string
		bitrate     string
		transcribe  bool
		model       string
		language    string
		keepMP3     bool
		help        bool
		downscale   bool
		videoOutput string
		resolution  string
		vcodec      string
		crf         int
		preset      string
		abitrate    string
	)

	flag.StringVar(&typeFile, "type", "", "Tipo mp4 ou mp3")
	flag.StringVar(&inputPath, "i", "", "Caminho do arquivo MP4 de entrada")
	flag.StringVar(&inputPath, "input", "", "Caminho do arquivo MP4 de entrada")
	flag.StringVar(&outputPath, "o", "", "Caminho do arquivo MP3 de saída (opcional)")
	flag.StringVar(&outputPath, "output", "", "Caminho do arquivo MP3 de saída (opcional)")
	flag.StringVar(&bitrate, "b", "192k", "Bitrate do MP3 (ex: 128k, 192k, 320k)")
	flag.StringVar(&bitrate, "bitrate", "192k", "Bitrate do MP3 (ex: 128k, 192k, 320k)")
	flag.BoolVar(&transcribe, "t", false, "Transcrever o áudio para texto")
	flag.BoolVar(&transcribe, "transcribe", false, "Transcrever o áudio para texto")
	flag.StringVar(&model, "m", "medium", "Modelo Whisper: tiny, base, small, medium, large")
	flag.StringVar(&model, "model", "medium", "Modelo Whisper: tiny, base, small, medium, large")
	flag.StringVar(&language, "l", "pt", "Idioma para transcrição (padrão: pt)")
	flag.StringVar(&language, "language", "pt", "Idioma para transcrição (padrão: pt)")
	flag.BoolVar(&keepMP3, "keep", true, "Manter arquivo MP3 após transcrição")
	flag.BoolVar(&help, "h", false, "Mostrar ajuda")
	flag.BoolVar(&help, "help", false, "Mostrar ajuda")
	flag.BoolVar(&downscale, "downscale", false, "Otimizar/reduzir o vídeo MP4")
	flag.StringVar(&videoOutput, "vo", "", "Caminho do vídeo otimizado de saída (opcional)")
	flag.StringVar(&videoOutput, "video-output", "", "Caminho do vídeo otimizado de saída (opcional)")
	flag.StringVar(&resolution, "r", "720p", "Resolução alvo: 360p, 480p, 720p, 1080p (padrão: 720p)")
	flag.StringVar(&resolution, "resolution", "720p", "Resolução alvo: 360p, 480p, 720p, 1080p (padrão: 720p)")
	flag.StringVar(&vcodec, "vcodec", "h264", "Codec de vídeo: h264, h265 (padrão: h264)")
	flag.IntVar(&crf, "crf", 23, "Fator de qualidade CRF (0-51, padrão: 23; menor = melhor qualidade)")
	flag.StringVar(&preset, "preset", "medium", "Preset de codificação: ultrafast, fast, medium, slow (padrão: medium)")
	flag.StringVar(&abitrate, "abitrate", "128k", "Bitrate do áudio no vídeo otimizado (padrão: 128k)")

	flag.Parse()

	if help || inputPath == "" {
		showHelp()
		return
	}

	if typeFile == "" && !downscale {
		log.Println("Tipo não especificado, assumindo MP4.")
		typeFile = "mp4"
	} else if typeFile == "mp3" && !transcribe {
		log.Println("Aviso: Tipo MP3 especificado, mas transcrição não está ativada. Apenas extração de áudio será realizada.")
	}

	// Verificar se o arquivo existe
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Printf("Erro: Arquivo não encontrado: %s\n", inputPath)
		os.Exit(1)
	}

	// Definir arquivo de saída se não especificado
	if outputPath == "" {
		dir := filepath.Dir(inputPath)
		base := filepath.Base(inputPath)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		outputPath = filepath.Join(dir, name+".mp3")
	}

	// Extrair FFmpeg se necessário (conversão de áudio ou downscale de vídeo)
	var ffmpegPath string
	if typeFile == "mp4" || downscale {
		var cleanup func()
		var err error
		ffmpegPath, cleanup, err = extractFFmpeg()
		if err != nil {
			fmt.Printf("Erro ao preparar FFmpeg: %v\n", err)
			os.Exit(1)
		}
		defer cleanup()
	}

	if typeFile == "mp4" {
		fmt.Printf("Extraindo áudio de: %s\n", inputPath)
		fmt.Printf("Salvando em: %s\n", outputPath)
		fmt.Printf("Bitrate: %s\n", bitrate)

		if err := convertToMP3(ffmpegPath, inputPath, outputPath, bitrate); err != nil {
			fmt.Printf("Erro ao converter: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Conversão concluída com sucesso!")
	}

	if downscale {
		if videoOutput == "" {
			dir := filepath.Dir(inputPath)
			base := filepath.Base(inputPath)
			name := strings.TrimSuffix(base, filepath.Ext(base))
			videoOutput = filepath.Join(dir, name+"_optimized.mp4")
		}

		fmt.Printf("\nOtimizando vídeo: %s\n", inputPath)
		fmt.Printf("Resolução alvo: %s | Codec: %s | CRF: %d | Preset: %s\n", resolution, vcodec, crf, preset)
		fmt.Printf("Salvando em: %s\n", videoOutput)

		if err := downscaleMP4Video(ffmpegPath, inputPath, videoOutput, resolution, vcodec, preset, abitrate, crf); err != nil {
			fmt.Printf("Erro ao otimizar vídeo: %v\n", err)
			os.Exit(1)
		}

		originalInfo, _ := os.Stat(inputPath)
		optimizedInfo, _ := os.Stat(videoOutput)
		if originalInfo != nil && optimizedInfo != nil {
			reduction := 100.0 - (float64(optimizedInfo.Size()) / float64(originalInfo.Size()) * 100.0)
			fmt.Printf("\nOtimização concluída!\n")
			fmt.Printf("Original:  %.2f MB\n", float64(originalInfo.Size())/1024/1024)
			fmt.Printf("Otimizado: %.2f MB\n", float64(optimizedInfo.Size())/1024/1024)
			fmt.Printf("Redução:   %.1f%%\n", reduction)
			fmt.Printf("Salvo em:  %s\n", videoOutput)
		} else {
			fmt.Println("\nOtimização de vídeo concluída!")
		}
	}

	// Se transcrição foi solicitada
	if transcribe {
		// Verificar se Whisper está instalado
		if !checkWhisper() {
			fmt.Println("\nErro: Whisper não está instalado!")
			fmt.Println("Instale com: pip install openai-whisper")
			fmt.Println("Mais informações: https://github.com/openai/whisper")
			os.Exit(1)
		}

		fmt.Println("\nIniciando transcrição local com Whisper...")
		fmt.Printf("Modelo: %s | Idioma: %s\n", model, language)
		fmt.Println("Nota: A primeira execução baixará o modelo automaticamente.")

		// Transcrever áudio
		transcriptionPath := strings.TrimSuffix(outputPath, ".mp3") + ".txt"
		if err := transcribeWithWhisper(outputPath, transcriptionPath, model, language); err != nil {
			fmt.Printf("Erro ao transcrever: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nTranscrição salva em: %s\n", transcriptionPath)

		// Remover MP3 se solicitado
		if !keepMP3 {
			os.Remove(outputPath)
			fmt.Println("Arquivo MP3 removido.")
		}
	}
}

func showHelp() {
	fmt.Println("MP4 para MP3 - Extrator, Transcritor e Otimizador de Vídeo (100% Offline)")
	fmt.Println("\nUso:")
	fmt.Println("  mp4tomp3 -i <arquivo.mp4> [opções]")
	fmt.Println("\nOpções gerais:")
	fmt.Println("  --type            Tipo da entrada (mp4 ou mp3, padrão: mp4)")
	fmt.Println("  -i, --input       Caminho do arquivo MP4 de entrada (obrigatório)")
	fmt.Println("  -h, --help        Mostrar esta ajuda")
	fmt.Println("\nExtração de áudio (MP4 → MP3):")
	fmt.Println("  -o, --output      Caminho do arquivo MP3 de saída (opcional)")
	fmt.Println("  -b, --bitrate     Bitrate do MP3 (padrão: 192k)")
	fmt.Println("\nTranscrição (requer Whisper):")
	fmt.Println("  -t, --transcribe  Transcrever o áudio para texto usando Whisper local")
	fmt.Println("  -m, --model       Modelo Whisper: tiny, base, small, medium, large (padrão: medium)")
	fmt.Println("  -l, --language    Idioma para transcrição (padrão: pt)")
	fmt.Println("  --keep            Manter arquivo MP3 após transcrição (padrão: true)")
	fmt.Println("\nDownscaling / Otimização de vídeo:")
	fmt.Println("  --downscale       Ativar otimização/redução do vídeo MP4")
	fmt.Println("  -r, --resolution  Resolução alvo: 360p, 480p, 720p, 1080p (padrão: 720p)")
	fmt.Println("  --vcodec          Codec de vídeo: h264, h265 (padrão: h264)")
	fmt.Println("  --crf             Qualidade CRF 0-51 (padrão: 23; menor = melhor qualidade)")
	fmt.Println("  --preset          Preset de codificação: ultrafast, fast, medium, slow (padrão: medium)")
	fmt.Println("  --abitrate        Bitrate do áudio no vídeo otimizado (padrão: 128k)")
	fmt.Println("  --vo, --video-output  Caminho do vídeo otimizado de saída (padrão: <nome>_optimized.mp4)")
	fmt.Println("\nExemplos:")
	fmt.Println("  # Apenas extrair áudio")
	fmt.Println("  mp4tomp3 -i video.mp4")
	fmt.Println("")
	fmt.Println("  # Otimizar vídeo para 720p (downscale)")
	fmt.Println("  mp4tomp3 -i video.mp4 --downscale")
	fmt.Println("")
	fmt.Println("  # Otimizar para 480p com h265 (menor arquivo)")
	fmt.Println("  mp4tomp3 -i video.mp4 --downscale -r 480p --vcodec h265")
	fmt.Println("")
	fmt.Println("  # Otimizar vídeo e também extrair áudio")
	fmt.Println("  mp4tomp3 -i video.mp4 --downscale --type mp4")
	fmt.Println("")
	fmt.Println("  # Extrair e transcrever com modelo medium")
	fmt.Println("  mp4tomp3 -i reuniao.mp4 -t")
	fmt.Println("")
	fmt.Println("  # Transcrever e remover MP3")
	fmt.Println("  mp4tomp3 -i video.mp4 -t --keep=false")
	fmt.Println("\nNotas sobre qualidade CRF (--crf):")
	fmt.Println("  h264: 0=lossless, 18=ótima, 23=boa (padrão), 28=aceitável")
	fmt.Println("  h265: valores similares, arquivo ~40% menor que h264")
	fmt.Println("\nRequisitos para transcrição:")
	fmt.Println("  - Python 3.8+ instalado")
	fmt.Println("  - Whisper instalado: pip install openai-whisper")
	fmt.Println("  - ~1-10GB de espaço (dependendo do modelo)")
	fmt.Println("\nTamanhos dos modelos Whisper:")
	fmt.Println("  tiny   : ~39MB  (mais rápido, menor qualidade)")
	fmt.Println("  base   : ~74MB")
	fmt.Println("  small  : ~244MB")
	fmt.Println("  medium : ~769MB (recomendado para português)")
	fmt.Println("  large  : ~1550MB (melhor qualidade)")
}

func extractFFmpeg() (string, func(), error) {
	// Determinar plataforma
	var ffmpegEmbed string
	switch runtime.GOOS {
	case "windows":
		ffmpegEmbed = "ffmpeg/windows/ffmpeg.exe"
	case "linux":
		ffmpegEmbed = "ffmpeg/linux/ffmpeg"
	case "darwin":
		ffmpegEmbed = "ffmpeg/darwin/ffmpeg"
	default:
		return "", nil, fmt.Errorf("plataforma não suportada: %s", runtime.GOOS)
	}

	// Criar diretório temporário
	tempDir, err := os.MkdirTemp("", "mp4tomp3-*")
	if err != nil {
		return "", nil, err
	}

	// Ler FFmpeg embarcado
	data, err := embeddedFFmpeg.ReadFile(ffmpegEmbed)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", nil, fmt.Errorf("FFmpeg não encontrado para esta plataforma: %v", err)
	}

	// Escrever FFmpeg no temp
	ffmpegPath := filepath.Join(tempDir, "ffmpeg")
	if runtime.GOOS == "windows" {
		ffmpegPath += ".exe"
	}

	if err := os.WriteFile(ffmpegPath, data, 0755); err != nil {
		os.RemoveAll(tempDir)
		return "", nil, err
	}

	// Função de limpeza
	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return ffmpegPath, cleanup, nil
}

func downscaleMP4Video(ffmpegPath, input, output, resolution, vcodec, preset, abitrate string, crf int) error {
	// Mapear resolução para filtro de escala do FFmpeg (preserva aspect ratio)
	var scale string
	switch resolution {
	case "360p":
		scale = "scale=-2:360"
	case "480p":
		scale = "scale=-2:480"
	case "720p":
		scale = "scale=-2:720"
	case "1080p":
		scale = "scale=-2:1080"
	default:
		// Aceita altura personalizada, ex: "540"
		scale = fmt.Sprintf("scale=-2:%s", resolution)
	}

	// Mapear codec de vídeo
	var videoCodec string
	switch vcodec {
	case "h265", "hevc":
		videoCodec = "libx265"
	default:
		videoCodec = "libx264"
	}

	args := []string{
		"-i", input,
		"-vf", scale,
		"-c:v", videoCodec,
		"-crf", fmt.Sprintf("%d", crf),
		"-preset", preset,
		"-c:a", "aac",
		"-b:a", abitrate,
		"-movflags", "+faststart",
		"-y",
		output,
	}

	cmd := exec.Command(ffmpegPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func convertToMP3(ffmpegPath, input, output, bitrate string) error {
	args := []string{
		"-i", input,
		"-vn",
		"-acodec", "mp3",
		"-ab", bitrate,
		"-ar", "44100",
		"-y",
		output,
	}

	cmd := exec.Command(ffmpegPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func checkWhisper() bool {

	cmd := exec.Command("whisper", "--help")
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

	output, err := cmd.CombinedOutput()

	log.Println("Verificando Whisper...")

	if err != nil {
		log.Printf("Output:\n%s", string(output))
		log.Println("Erro:", err)
	}
	return err == nil
}

func transcribeWithWhisper(audioPath, outputPath, model, language string) error {
	//audioPath = strings.ReplaceAll(audioPath, `\`, `/`)
	//outputPath = strings.ReplaceAll(outputPath, `\`, `/`)

	log.Println("audioPath ", audioPath)

	// Preparar comando Whisper
	args := []string{
		audioPath,
		"--model", model,
		"--language", language,
		"--output_format", "txt",
		"--output_dir", filepath.Dir(outputPath),
		"--verbose", "True",
		"--task", "transcribe",
	}

	// Para reuniões longas, adicionar configurações otimizadas
	if model == "large" {
		args = append(args, "--beam_size", "5")
	}

	cmd := exec.Command("whisper", args...)

	// Criar pipes para capturar saída
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	// Iniciar comando
	if err := cmd.Start(); err != nil {
		return err
	}

	// Ler saída em tempo real
	scanner := bufio.NewScanner(stdout)
	go func() {
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	scannerErr := bufio.NewScanner(stderr)
	go func() {
		for scannerErr.Scan() {
			line := scannerErr.Text()
			// Filtrar apenas mensagens importantes
			if strings.Contains(line, "%") || strings.Contains(line, "Detecting") || strings.Contains(line, "Transcribing") {
				fmt.Println(line)
			}
		}
	}()

	// Aguardar conclusão
	if err := cmd.Wait(); err != nil {
		return err
	}

	// Whisper gera arquivo com nome base do áudio
	whisperOutput := strings.TrimSuffix(audioPath, filepath.Ext(audioPath)) + ".txt"

	// Se o arquivo de saída desejado for diferente, renomear
	if whisperOutput != outputPath {
		return os.Rename(whisperOutput, outputPath)
	}

	return nil
}

// ===== ALTERNATIVA: Script Python embutido para mais controle =====

func generateWhisperScript() string {
	return `
import whisper
import sys
import os

def transcribe_audio(audio_path, model_name="medium", language="pt"):
    """
    Transcreve áudio usando Whisper com configurações otimizadas para português
    """
    print(f"Carregando modelo {model_name}...")
    model = whisper.load_model(model_name)
    
    print(f"Transcrevendo {audio_path}...")
    print("Isso pode demorar para arquivos longos...")
    
    # Transcrever com configurações otimizadas
    result = model.transcribe(
        audio_path,
        language=language,
        task="transcribe",
        verbose=True,
        # Configurações para melhor qualidade em português
        beam_size=5 if model_name == "large" else None,
        best_of=5,
        temperature=0.0,  # Reduz "criatividade" para maior precisão
        compression_ratio_threshold=2.4,
        logprob_threshold=-1.0,
        no_speech_threshold=0.6,
        condition_on_previous_text=True,
        initial_prompt="Transcrição em português brasileiro.",
    )
    
    # Salvar transcrição
    output_path = os.path.splitext(audio_path)[0] + ".txt"
    with open(output_path, "w", encoding="utf-8") as f:
        f.write(result["text"])
    
    print(f"\nTranscrição salva em: {output_path}")
    
    # Estatísticas
    if result.get("language"):
        print(f"Idioma detectado: {result['language']}")
    
    return output_path

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Uso: python transcribe.py <audio_file> [model] [language]")
        sys.exit(1)
    
    audio_file = sys.argv[1]
    model = sys.argv[2] if len(sys.argv) > 2 else "medium"
    language = sys.argv[3] if len(sys.argv) > 3 else "pt"
    
    transcribe_audio(audio_file, model, language)
`
}

/*
INSTRUÇÕES PARA CONFIGURAR WHISPER LOCALMENTE:

1. Instalar Python (3.8 ou superior)
   - Windows: https://www.python.org/downloads/
   - Linux: sudo apt install python3 python3-pip
   - macOS: brew install python3

2. Instalar Whisper:
   pip install openai-whisper

3. (Opcional) Para melhor performance com GPU NVIDIA:
   pip install torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu118

4. Primeira execução baixará o modelo automaticamente:
   - tiny: ~1 minuto para download
   - medium: ~5 minutos para download
   - large: ~10 minutos para download

DICAS PARA TRANSCRIÇÃO DE REUNIÕES LONGAS:

- Use o modelo "medium" para bom equilíbrio entre qualidade e velocidade
- Para reuniões muito importantes, use "large" (mais lento mas mais preciso)
- Em máquinas com pouca RAM (< 8GB), use "small" ou "base"
- Whisper processa em chunks de 30 segundos automaticamente
- Uma reunião de 1 hora pode levar:
  - tiny: ~5 minutos
  - medium: ~15-20 minutos
  - large: ~30-60 minutos (dependendo do hardware)

OTIMIZAÇÕES:

- Para usar GPU NVIDIA (muito mais rápido):
  Certifique-se de ter CUDA instalado e torch com suporte GPU

- Para máquinas com múltiplos cores:
  Whisper usa automaticamente todos os cores disponíveis
*/
