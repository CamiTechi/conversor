package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	fyneDialog "fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	nativeDialog "github.com/sqweek/dialog"
)

var arqselec string
var bitesc string = "192k"
var btnCon *widget.Button
var mp3selec string
var modeloWhisper string = "medium"
var transcreverAutomatico bool

func runGUI() {
	fmt.Println("")

	a := app.New()
	w := a.NewWindow("SETI Conversor")
	w.Resize(fyne.NewSize(680, 580))

	os.Setenv("PYTHONWCASE", "1") //avisa o interpretador do Python para rodar sem janela

	//extração do ffmpeg embutido
	ffmpegPath, cleanup, err := extractFFmpeg()
	if err == nil {
		w.SetOnClosed(func() {
			cleanup()
		})

		ffmpegDir := filepath.Dir(ffmpegPath)
		caminhoAtual := os.Getenv("PATH")

		//busca o caminho do python
		userHome, _ := os.UserHomeDir()
		basePythonPath := filepath.Join(userHome, "AppData", "Local", "Python")

		var pythonScriptsDir string
		var pythonBaseDir string

		//vasculha a pasta do Python do usuário para achar a versão instalada
		if arquivos, err := os.ReadDir(basePythonPath); err == nil {
			for _, arquivo := range arquivos {
				if arquivo.IsDir() && strings.HasPrefix(arquivo.Name(), "pythoncore-") {
					//encontra a pasta do pyhton
					pythonBaseDir = filepath.Join(basePythonPath, arquivo.Name())
					pythonScriptsDir = filepath.Join(pythonBaseDir, "Scripts")
					break
				}
			}
		}

		//monta o PATH com o que foi encontrado + FFmpeg embutido
		novoPath := ffmpegDir
		if pythonScriptsDir != "" {
			novoPath += ";" + pythonScriptsDir + ";" + pythonBaseDir
		}
		novoPath += ";" + caminhoAtual
		os.Setenv("PATH", novoPath)
	}

	os.Setenv("COMSPEC", "") //força o ambiente do windows a esconder janelas de comandos

	labelArquivo := widget.NewLabel("Nenhum arquivo selecionado")
	labelArquivo.Wrapping = fyne.TextWrapWord

	labelStatus := widget.NewLabel("")
	labelStatus.Wrapping = fyne.TextWrapWord

	labelStatusCon := widget.NewLabel("")
	labelStatusCon.Wrapping = fyne.TextWrapWord

	//dropdown
	bitselec := widget.NewSelect(
		[]string{"128k", "192k", "256k", "320k"},
		func(valor string) {
			bitesc = valor
		},
	)
	bitselec.SetSelected("192k")

	mosec := widget.NewSelect( //dropdown do modelo whisper
		[]string{"tiny", "small", "medium", "large"},
		func(valor string) {
			modeloWhisper = valor
		},
	)
	mosec.SetSelected("medium")
	mosec.Hide() // esconde o dropdown

	mosec2 := widget.NewSelect( //dropdown do modelo whisper
		[]string{"tiny", "small", "medium", "large"},
		func(valor string) {
			modeloWhisper = valor
		},
	)
	mosec2.SetSelected("medium")

	//CHECKBOX PARA O ARQUIVO .TXT
	checkbox1 := widget.NewCheck("Transcrever após converter", func(marcado bool) { //widget.newCheck("", func(x bool){}) pra criar um checkbox
		transcreverAutomatico = marcado

		if marcado {
			mosec.Show() // mostra o dropdown se a checkbox for marcada
		} else {
			mosec.Hide()
		}
		w.Content().Refresh() // atualiza a interface para refletir as mudanças
	})

	progress := widget.NewProgressBar()
	progress.Hide()

	progress2 := widget.NewProgressBar()
	progress2.Hide()

	labelMP3 := widget.NewLabel("Nenhum arquivo selecionado")
	labelMP3.Wrapping = fyne.TextWrapWord

	stopChan := make(chan struct{})
	btnStopTrans := widget.NewButton("Parar", func() {
		close(stopChan)
	})
	btnStopTrans.Hide()

	btnStopCon := widget.NewButton("Parar", func() {
		close(stopChan)
	})
	btnStopCon.Hide()

	var btnTranscrever *widget.Button

	//abre explorador de arquivos pra selecionar o arquivo mp3 a ser transcrito
	btnSelecMP3 := widget.NewButtonWithIcon(
		"Selecionar arquivo",
		theme.FolderOpenIcon(),
		func() {
			filename, err := nativeDialog.File().
				Filter("Arquivos de áudio e vídeo (MP3, WAV, OGG, AIFF, AAC...)", "mp3", "wav", "ogg", "aiff", "aac").
				Load()
			if err != nil || filename == "" {
				return
			}
			mp3selec = filename
			labelMP3.SetText("📄" + filepath.Base(mp3selec))
			labelStatus.SetText("")
		},
	)

	btnLimpar := widget.NewButtonWithIcon(
		"Limpar seleção",
		theme.DeleteIcon(),
		func() {
			mp3selec = ""
			labelMP3.SetText("Nenhum arquivo selecionado")
		})

	btnTranscrever = widget.NewButtonWithIcon(
		"Transcrever arquivo para Texto",
		theme.DocumentIcon(),
		func() {

			//validação, verifica se há um arquivo para transcrever
			if mp3selec == "" {
				labelStatus.SetText("Selecione um arquivo primeiro.")
				return
			}

			//validação do sistema para verificar se o Whisper está instalado
			if !checkWhisper() {
				labelStatus.SetText("Whisper não instalado! Abra o terminal e rode: pip install openai-whisper.\nCertifique-se de ter o Python e pip configurados!\nPasso a passo:\n1. Instale o Python: https://www.python.org/downloads/\n2. Abra o terminal e rode: pip install openai-whisper\n3. Volte aqui e tente novamente.")
				return
			}
			// Define onde o arquivo txt vai ser salvo (mesma pasta e nome do arquivo selecionado)
			nomeSugerido := strings.TrimSuffix(filepath.Base(mp3selec), filepath.Ext(mp3selec)) + ".txt"

			//abre janela para escolher onde salvar o arquivo de texto, com o nome sugerido já preenchido
			saidaEsperada, err := nativeDialog.File().
				Title("Escolha onde salvar o arquivo de texto").
				Filter("Arquivo de texto (*.txt)*", "txt").
				SetStartFile(nomeSugerido). //coloca nome sugerido na caixinha
				Save()

			if err != nil || saidaEsperada == "" {
				labelStatus.SetText("Operação cancelada.")
				return
			}

			if !strings.HasSuffix(saidaEsperada, ".txt") {
				saidaEsperada = saidaEsperada + ".txt"
				//garante que o arquivo de saída tenha a extensão .txt mesmo que o usuário não a coloque, para evitar erros na hora de salvar a transcrição
			}

			stopChan = make(chan struct{})

			//feedback visual
			progress2.Show()
			progressoAnimado(progress2, stopChan)

			btnStopTrans.Show()
			btnTranscrever.Disable()
			btnCon.Disable() // Desabilita o botão de converter para evitar conflitos
			labelStatus.SetText("Transcrevendo áudio...")

			//dispara em segundo plano usando a função do back-end
			go func() {
				err := transcribeWithWhisper(mp3selec, saidaEsperada, modeloWhisper, "pt")

				//retorna a resposta para a interface gráfica
				fyne.Do(func() {
					close(stopChan)
					progress2.SetValue(1.0) //garante que a barra de progresso fique completa ao finalizar

					if err != nil {
						labelStatus.SetText("Erro na transcrição: " + err.Error())
					} else {
						labelStatus.SetText(fmt.Sprintf("Transcrição concluída! Salvo como: \"%s\" em: \"%s\"", filepath.Base(saidaEsperada), filepath.Dir(saidaEsperada)))

						mp3selec = ""
						labelMP3.SetText("Nenhum arquivo selecionado")

					}
					progress2.Hide()
					btnStopTrans.Hide()
					btnTranscrever.Enable()
					btnCon.Enable() // Reabilita o botão de converter após a transcrição
				})
			}()
		})

	//BOTÃO SELECIONAR
	btnSelecionar := widget.NewButtonWithIcon(
		"Selecionar arquivo",
		theme.FolderOpenIcon(),
		func() {
			filename, err := nativeDialog.File().
				Filter("Arquivos suportados (MP4, WAV, OGG, AIFF, AAC...)", "mp4", "wav", "ogg", "aiff", "aac").
				Load()
			if err != nil || filename == "" {
				return
			}

			arqselec = filename
			labelArquivo.SetText("📄" + filepath.Base(arqselec))
			labelStatus.SetText("")
		})
	btnLimparConversao := widget.NewButtonWithIcon(
		"Limpar seleção",
		theme.DeleteIcon(),
		func() {
			arqselec = ""
			labelArquivo.SetText("Nenhum arquivo selecionado")
		})

	//BOTÃO CONVERTER
	btnCon = widget.NewButtonWithIcon(
		"Converter para MP3",
		theme.MediaPlayIcon(),
		func() {
			if arqselec == "" {
				labelStatusCon.SetText("Selecione um arquivo para converter.")
				return
			}

			nomeSugerido := strings.TrimSuffix(filepath.Base(arqselec), filepath.Ext(arqselec)) + ".mp3"

			saida, err := nativeDialog.File().
				Title("Escolha onde salvar o arquivo MP3").
				Filter("Arquivo MP3 (*.mp3)*", "mp3").
				SetStartFile(nomeSugerido). //coloca nome sugerido na caixinha
				Save()
			if err != nil || saida == "" {
				labelStatus.SetText("Operação cancelada.")
				return
			}

			if !strings.HasSuffix(saida, ".mp3") {
				saida = saida + ".mp3"
				//garante que o arquivo de saída tenha a extensão .mp3 mesmo que o usuário não a coloque, para evitar erros na conversão e na transcrição automática depois
			}

			stopChan = make(chan struct{})

			// saida := strings.TrimSuffix(arqselec, filepath.Ext(arqselec)) + ".mp3"
			progress.Show()
			progressoAnimado(progress, stopChan)

			btnStopCon.Show()
			btnCon.Disable()
			btnTranscrever.Disable() //desabilita o botão de transcrever para evitar conflitos

			labelStatusCon.SetText("Processando...")

			go func() {
				err = convertToMP3(ffmpegPath, arqselec, saida, bitesc)

				fyne.Do(func() {
					if !transcreverAutomatico {
						close(stopChan)
						progress.SetValue(1.0)
					}
					labelStatusCon.SetText("Conversão concluída!")
				})

				if transcreverAutomatico {
					err = convertToMP3(ffmpegPath, arqselec, saida, bitesc)

					fyne.Do(func() {
						close(stopChan)
						progress.SetValue(1.0)
					})
				}
				ffmpegPath, cleanup, err := extractFFmpeg()
				if err != nil {
					fyne.Do(func() {
						labelStatus.SetText("Erro ao preparar FFmpeg: " + err.Error())
						progress.Hide()
						btnStopCon.Hide()
						btnCon.Enable()
						btnTranscrever.Enable() //reabilita o botão de transcrever mesmo se der erro no FFmpeg
					})
					return
				}
				defer cleanup()

				err = convertToMP3(ffmpegPath, arqselec, saida, bitesc)

				if err != nil {
					fyne.Do(func() {
						labelStatus.SetText("Erro na conversão: " + err.Error())
						progress.Hide()
						btnStopCon.Hide()
						btnCon.Enable()
						btnTranscrever.Enable()
					})
					return
				}
				arqconvertido := arqselec
				fyne.Do(func() {
					labelStatusCon.SetText("Conversão concluída!\nArquivo: " + filepath.Base(saida) + " Salvo em: " + filepath.Dir(saida))

					arqselec = ""
					labelArquivo.SetText("Nenhum arquivo selecionado")
					_ = arqconvertido
				})

				if transcreverAutomatico {
					fyne.Do(func() {
						labelStatusCon.SetText("Iniciando transcrição automática...")
					})

					if !checkWhisper() {
						fyne.Do(func() {
							fyneDialog.ShowError(errors.New("Whisper não instalado! Abra o terminal e rode: pip install openai-whisper.\nCertifique-se de ter o Python e pip configurados!\nPasso a passo:\n1. Instale o Python: https://www.python.org/downloads/\n2. Abra o terminal e rode: pip install openai-whisper\n3. Volte aqui e tente novamente."), w)
							progress.Hide()
							btnStopCon.Hide()
							btnTranscrever.Enable()
							btnCon.Enable()
						})
						return
					}

					txtSaida := strings.TrimSuffix(saida, filepath.Ext(saida)) + ".txt" //analisa o caminho do arquivo e extrai dinamicamente tudo o que estiver após o
					//último ponto. Ao usar o strings.TrimSuffix, limpa a extensão e garante que o arquivo de texto final se chame estritamente .txt
					err = transcribeWithWhisper(saida, txtSaida, modeloWhisper, "pt")

					err = transcribeWithWhisper(saida, txtSaida, modeloWhisper, "pt")

					fyne.Do(func() {
						if err != nil {
							labelStatusCon.SetText("Erro na transcrição automática: " + err.Error())
						} else {
							labelStatusCon.SetText(fmt.Sprintf("Conversão e Transcrição concluídas!\nSalvos como: \"%s\" e \"%s\" em: \"%s\"", filepath.Base(saida), filepath.Base(txtSaida), filepath.Dir(saida)))
							mp3selec = ""
							labelMP3.SetText("Nenhum arquivo selecionado")
						}
						progress.Hide()
						btnStopCon.Hide()
						btnTranscrever.Enable()
						btnCon.Enable()
					})
					_ = arqconvertido
					return
				}

				fyne.Do(func() {
					progress.Hide()

					btnStopCon.Hide()
					btnCon.Enable()
					btnTranscrever.Enable() // Reabilita o botão de transcrever após a conversão, para que o usuário possa escolher transcrever o arquivo convertido ou outro arquivo MP3 que já tenha
				})
			}()
		},
	)

	btnHelp := widget.NewButtonWithIcon(
		"Ajuda",
		theme.HelpIcon(),
		func() {
			fyneDialog.ShowInformation(
				"Instruções de Uso", "1. Para converter um arquivo de áudio ou vídeo para MP3, clique em 'Selecionar arquivo', escolha seu arquivo (MP4, WAV, OGG, AIFF, AAC...) e depois clique em 'Converter para MP3'.\n\n2. Para transcrever um arquivo MP3 já existente para texto, clique em 'Selecione um arquivo MP3', escolha seu arquivo e depois clique em 'Transcrever arquivo para Texto'.\n\n3. Você pode escolher a qualidade do MP3 usando o menu de seleção (bitrate) antes de converter.\n\n4. Se quiser que o programa transcreva automaticamente após converter para MP3, marque a opção correspondente.\n\nTRANSCRIÇÃO DE ÁUDIO PARA TEXTO, instale:\n\n1. Python: https://www.python.org/downloads/\n2. Abra o terminal e rode: pip install openai-whisper\n3. Volte aqui e tente novamente.",
				w,
			)
		},
	)
	btnHelp.Importance = widget.HighImportance

	btnAtivo := false

	btnAtivo = !btnAtivo
	if btnAtivo {
		btnHelp.Show()
	} else {
		btnHelp.Hide()
	}
	btnHelp.Refresh()

	card := widget.NewCard(
		"Dica!",
		"",
		widget.NewLabel("- Use o modelo 'medium' para bom equilíbrio entre qualidade e velocidade.\n- Para reuniões muito importantes, use 'large' (mais lento mas mais preciso).\n- Em máquinas com pouco RAM (< 8GB), use 'small' ou 'base.\n- Uma reunião de 1 hora pode levar:\n\t- Tiny: ~5 minutos\n\t- Medium: ~15-20 minutos\n\t- Large: ~30-60 minutos"),
	)
	card.Hide()

	var content *fyne.Container
	var btnDica *widget.Button

	btnDica = widget.NewButtonWithIcon(
		"Mostrar dica",
		theme.InfoIcon(),
		func() {
			if card.Visible() {
				card.Hide()
				btnDica.SetText("Mostrar Dica")

				// content.Refresh()
				w.Resize(fyne.NewSize(680, 580))
			} else {
				card.Show()
				btnDica.SetText("Esconder Dica")

				content.Refresh()

				w.Resize(fyne.NewSize(680, 580))
			}
			w.Content().Refresh() //força o container a reajustar para dar espaço ao texto
		},
	)
	btnDica.Importance = widget.HighImportance

	labelBitrate := widget.NewLabel("Qualidade (bitrate):")
	labelBitrate.Wrapping = fyne.TextWrapWord

	btnDicaAjudaEDicaCentralizado := container.NewHBox(
		layout.NewSpacer(),
		btnHelp,
		btnDica,
		layout.NewSpacer(),
	)

	//Visualização
	content = container.NewVBox(
		widget.NewRichTextFromMarkdown("# Arquivo de Entrada:"),
		// container.NewGridWithColumns(2, btnSelecionar, btnLimparConversao),
		container.NewGridWithColumns(3, labelArquivo, btnSelecionar, btnLimparConversao),

		// widget.NewSeparator(),
		labelBitrate,
		bitselec,

		// widget.NewSeparator(),
		checkbox1,
		mosec,
		btnCon,
		progress,
		labelStatusCon,

		widget.NewSeparator(),
		widget.NewRichTextFromMarkdown("# Transcrição de Áudio para Texto:"),
		container.NewGridWithColumns(3, labelMP3, btnSelecMP3, btnLimpar),
		widget.NewLabel("Tamanho (precisão): "),
		mosec2,
		btnTranscrever,
		labelStatus,
		progress2,

		widget.NewSeparator(),
		btnDicaAjudaEDicaCentralizado,
		card,
	)
	padded := container.NewPadded(content)
	w.SetContent(padded)
	w.ShowAndRun()
}

func progressoAnimado(barra *widget.ProgressBar, stopChan chan struct{}) {
	barra.SetValue(0)
	go func() {
		for i := 0; i <= 100; i++ {
			select {
			case <-stopChan:
				return
			default:
				time.Sleep(time.Millisecond * 300) //velocidade do carregamento
				barra.SetValue(float64(i) / 100.0)
			}
		}
	}()
}
