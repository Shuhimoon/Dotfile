# dotfiles:go-path
case ":$PATH:" in
  *:/usr/local/go/bin:*) ;;
  *) export PATH="$PATH:/usr/local/go/bin" ;;
esac
