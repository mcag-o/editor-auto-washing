import AccountTreeOutlinedIcon from '@mui/icons-material/AccountTreeOutlined';
import AutoAwesomeOutlinedIcon from '@mui/icons-material/AutoAwesomeOutlined';
import Box from '@mui/material/Box';
import Chip from '@mui/material/Chip';
import Container from '@mui/material/Container';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';

export default function App() {
  return (
    <Box sx={{ minHeight: '100vh', py: 8 }}>
      <Container maxWidth="md">
        <Paper
          elevation={0}
          sx={{
            borderRadius: 4,
            border: '1px solid',
            borderColor: 'divider',
            px: { xs: 3, sm: 6 },
            py: { xs: 4, sm: 6 },
            background: 'linear-gradient(180deg, #ffffff 0%, #f7f9fc 100%)',
          }}
        >
          <Stack spacing={3}>
            <Stack direction="row" spacing={1.5} alignItems="center">
              <AutoAwesomeOutlinedIcon color="primary" />
              <Chip label="React + Vite scaffold" color="primary" variant="outlined" />
            </Stack>
            <Typography variant="h3" component="h1" sx={{ fontWeight: 700 }}>
              Content Hub Control Plane
            </Typography>
            <Typography variant="body1" color="text.secondary">
              Web control plane scaffold is now served by the Go binary. Feature pages,
              workflow editing, and API integration will land in follow-up tasks.
            </Typography>
            <Stack direction="row" spacing={1.5} alignItems="center">
              <AccountTreeOutlinedIcon fontSize="small" color="action" />
              <Typography variant="body2" color="text.secondary">
                Material UI and React Flow dependencies are wired and ready for the next slice.
              </Typography>
            </Stack>
          </Stack>
        </Paper>
      </Container>
    </Box>
  );
}
